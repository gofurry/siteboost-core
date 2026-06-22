//go:build windows

package privilege

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func IsElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

func HasSystemPrivileges() bool {
	admin, err := isAdministratorToken()
	return err == nil && IsElevated() && admin
}

func RelaunchElevated(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable for elevated relaunch: %w", err)
	}
	cmdline := windows.ComposeCommandLine(args)
	cwd, _ := os.Getwd()
	if err := shellExecuteRunas(exe, cmdline, cwd); err != nil {
		if errors.Is(err, windows.ERROR_CANCELLED) {
			return fmt.Errorf("administrator authorization was canceled")
		}
		return fmt.Errorf("start elevated process: %w", err)
	}
	return nil
}

func runElevatedHelper(ctx context.Context, req HelperRequest) (HelperResponse, error) {
	helperCtx, cancel, err := helperContext(ctx)
	if err != nil {
		return HelperResponse{}, err
	}
	defer cancel()

	dir, err := os.MkdirTemp("", "steam-accelerator-helper-*")
	if err != nil {
		return HelperResponse{}, fmt.Errorf("create helper temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	token, err := randomToken()
	if err != nil {
		return HelperResponse{}, err
	}
	req.Version = helperVersion
	req.Token = token
	req.ParentPID = os.Getpid()

	requestPath := filepath.Join(dir, "request.json")
	responsePath := filepath.Join(dir, "response.json")
	if err := writeHelperRequest(requestPath, req); err != nil {
		return HelperResponse{}, err
	}

	exe, err := os.Executable()
	if err != nil {
		return HelperResponse{}, fmt.Errorf("locate executable for elevated helper: %w", err)
	}
	args := windows.ComposeCommandLine([]string{
		"__helper",
		"--request", requestPath,
		"--response", responsePath,
		"--token", token,
		"--parent", strconv.Itoa(req.ParentPID),
	})
	cwd, _ := os.Getwd()
	if err := shellExecuteRunas(exe, args, cwd); err != nil {
		if errors.Is(err, windows.ERROR_CANCELLED) {
			return HelperResponse{}, fmt.Errorf("elevated helper authorization was canceled")
		}
		return HelperResponse{}, fmt.Errorf("start elevated helper: %w", err)
	}
	return waitForHelperResponse(helperCtx, responsePath)
}

func shellExecuteRunas(exe, args, cwd string) error {
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	params, err := windows.UTF16PtrFromString(args)
	if err != nil {
		return err
	}
	var cwdPtr *uint16
	if cwd != "" {
		cwdPtr, err = windows.UTF16PtrFromString(cwd)
		if err != nil {
			return err
		}
	}
	return windows.ShellExecute(0, verb, file, params, cwdPtr, windows.SW_SHOWNORMAL)
}

func helperStatus() string {
	admin, adminErr := isAdministratorToken()
	integrity, integrityErr := integrityLevel()
	if adminErr != nil {
		admin = false
	}
	if integrityErr != nil {
		integrity = "unknown"
	}
	return fmt.Sprintf("helper pid=%d elevated=%t admin=%t integrity=%s", os.Getpid(), IsElevated(), admin, integrity)
}

func isAdministratorToken() (bool, error) {
	adminSID, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return false, err
	}
	return windows.Token(0).IsMember(adminSID)
}

func integrityLevel() (string, error) {
	var size uint32
	err := windows.GetTokenInformation(windows.GetCurrentProcessToken(), windows.TokenIntegrityLevel, nil, 0, &size)
	if err != nil && !errors.Is(err, windows.ERROR_INSUFFICIENT_BUFFER) {
		return "", err
	}
	if size == 0 {
		return "", fmt.Errorf("token integrity level size is zero")
	}
	buf := make([]byte, size)
	if err := windows.GetTokenInformation(windows.GetCurrentProcessToken(), windows.TokenIntegrityLevel, &buf[0], uint32(len(buf)), &size); err != nil {
		return "", err
	}
	label := (*windows.Tokenmandatorylabel)(unsafe.Pointer(&buf[0]))
	if label.Label.Sid == nil || label.Label.Sid.SubAuthorityCount() == 0 {
		return "", fmt.Errorf("token integrity SID is empty")
	}
	rid := label.Label.Sid.SubAuthority(uint32(label.Label.Sid.SubAuthorityCount() - 1))
	switch {
	case rid >= 0x4000:
		return "system", nil
	case rid >= 0x3000:
		return "high", nil
	case rid >= 0x2000:
		return "medium", nil
	case rid >= 0x1000:
		return "low", nil
	default:
		return fmt.Sprintf("rid-0x%x", rid), nil
	}
}

func waitForHelperResponse(ctx context.Context, responsePath string) (HelperResponse, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return HelperResponse{}, fmt.Errorf("elevated helper timed out or was canceled: %w", ctx.Err())
		case <-ticker.C:
			resp, err := readHelperResponse(responsePath)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return HelperResponse{}, err
			}
			if !resp.OK {
				if resp.Error == "" {
					resp.Error = "helper failed"
				}
				return resp, fmt.Errorf("elevated helper: %s", resp.Error)
			}
			return resp, nil
		}
	}
}

func randomToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate helper token: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}
