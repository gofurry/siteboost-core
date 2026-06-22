//go:build windows

package privilege

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const appHostServiceName = "SiteBoostCoreAppHost"

func IsElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

func HasSystemPrivileges() bool {
	if integrity, err := integrityLevel(); err == nil && integrity == "system" {
		return true
	}
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

func ensureAppHostStarted(ctx context.Context) error {
	if err := appHostHealth(ctx); err == nil {
		return nil
	}
	if err := StartAppHostService(ctx); err != nil {
		return fmt.Errorf("windows apphost is not running; install it once with `steam-accelerator apphost install` from an Administrator terminal: %w", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := appHostHealth(ctx); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("windows apphost service started but did not become ready: %w", lastErr)
}

func appHostHealth(ctx context.Context) error {
	reqCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, "http://"+appHostAddr+"/healthz", nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected apphost health status: %s", resp.Status)
	}
	return nil
}

func RunAppHostService(args []string, stdout, stderr io.Writer) error {
	_ = args
	isService, err := svc.IsWindowsService()
	if err != nil {
		return fmt.Errorf("detect windows service mode: %w", err)
	}
	if isService {
		return svc.Run(appHostServiceName, windowsAppHostService{})
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	fmt.Fprintln(stdout, "running windows apphost in console mode")
	if err := RunAppHostConsole(ctx, stdout); err != nil {
		fmt.Fprintf(stderr, "windows apphost stopped: %v\n", err)
		return err
	}
	return nil
}

type windowsAppHostService struct{}

func (windowsAppHostService) Execute(args []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	_ = args
	status <- svc.Status{State: svc.StartPending}
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- RunAppHostConsole(ctx, io.Discard)
	}()
	status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for {
		select {
		case req := <-requests:
			switch req.Cmd {
			case svc.Interrogate:
				status <- req.CurrentStatus
			case svc.Stop, svc.Shutdown:
				status <- svc.Status{State: svc.StopPending}
				cancel()
				err := <-errCh
				if err != nil {
					return false, 1
				}
				return false, 0
			default:
				status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
			}
		case err := <-errCh:
			cancel()
			if err != nil {
				return false, 1
			}
			return false, 0
		}
	}
}

func InstallAppHostService(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable for apphost service: %w", err)
	}
	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect service manager: %w", err)
	}
	defer manager.Disconnect()

	service, err := manager.OpenService(appHostServiceName)
	if err == nil {
		query, queryErr := service.Query()
		if err := configureAppHostService(service, exe); err != nil {
			_ = service.Close()
			return err
		}
		_ = service.Close()
		if queryErr == nil && query.State == svc.Running {
			if err := StopAppHostService(ctx); err != nil {
				return fmt.Errorf("restart apphost service after config update: %w", err)
			}
		}
		return StartAppHostService(ctx)
	}
	if !errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return fmt.Errorf("open apphost service: %w", err)
	}
	service, err = manager.CreateService(appHostServiceName, exe, newAppHostServiceConfig(""), "__apphost-service")
	if err != nil {
		return fmt.Errorf("create apphost service: %w", err)
	}
	defer service.Close()
	return StartAppHostService(ctx)
}

func configureAppHostService(service *mgr.Service, exe string) error {
	cfg, err := service.Config()
	if err != nil {
		return fmt.Errorf("query apphost service config: %w", err)
	}
	next := cfg
	desired := newAppHostServiceConfig(appHostServiceCommandLine(exe))
	next.DisplayName = desired.DisplayName
	next.Description = desired.Description
	next.StartType = desired.StartType
	next.DelayedAutoStart = desired.DelayedAutoStart
	next.BinaryPathName = desired.BinaryPathName
	if next.ServiceType == 0 {
		next.ServiceType = windows.SERVICE_WIN32_OWN_PROCESS
	}
	if next.ErrorControl == 0 {
		next.ErrorControl = mgr.ErrorNormal
	}
	if err := service.UpdateConfig(next); err != nil {
		return fmt.Errorf("update apphost service config: %w", err)
	}
	return nil
}

func newAppHostServiceConfig(binaryPathName string) mgr.Config {
	return mgr.Config{
		ServiceType:      windows.SERVICE_WIN32_OWN_PROCESS,
		StartType:        mgr.StartAutomatic,
		ErrorControl:     mgr.ErrorNormal,
		BinaryPathName:   binaryPathName,
		DisplayName:      "SiteBoost Core AppHost",
		Description:      "Privileged local apphost for SiteBoost Core hosts and certificate system changes.",
		DelayedAutoStart: true,
	}
}

func appHostServiceCommandLine(exe string) string {
	return syscall.EscapeArg(exe) + " " + syscall.EscapeArg("__apphost-service")
}

func UninstallAppHostService(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_ = StopAppHostService(ctx)
	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect service manager: %w", err)
	}
	defer manager.Disconnect()
	service, err := manager.OpenService(appHostServiceName)
	if err != nil {
		return fmt.Errorf("open apphost service: %w", err)
	}
	defer service.Close()
	if err := service.Delete(); err != nil {
		return fmt.Errorf("delete apphost service: %w", err)
	}
	return nil
}

func StartAppHostService(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect service manager: %w", err)
	}
	defer manager.Disconnect()
	service, err := manager.OpenService(appHostServiceName)
	if err != nil {
		return fmt.Errorf("open apphost service: %w", err)
	}
	defer service.Close()
	query, err := service.Query()
	if err == nil && query.State == svc.Running {
		return nil
	}
	if err := service.Start(); err != nil && !strings.Contains(strings.ToLower(err.Error()), "already") {
		return fmt.Errorf("start apphost service: %w", err)
	}
	return waitForServiceState(ctx, service, svc.Running, 10*time.Second)
}

func StopAppHostService(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect service manager: %w", err)
	}
	defer manager.Disconnect()
	service, err := manager.OpenService(appHostServiceName)
	if err != nil {
		return fmt.Errorf("open apphost service: %w", err)
	}
	defer service.Close()
	query, err := service.Query()
	if err == nil && query.State == svc.Stopped {
		return nil
	}
	_, err = service.Control(svc.Stop)
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "not active") {
		return fmt.Errorf("stop apphost service: %w", err)
	}
	return waitForServiceState(ctx, service, svc.Stopped, 10*time.Second)
}

func AppHostServiceStatus(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	manager, err := mgr.Connect()
	if err != nil {
		return "", fmt.Errorf("connect service manager: %w", err)
	}
	defer manager.Disconnect()
	service, err := manager.OpenService(appHostServiceName)
	if err != nil {
		return "", fmt.Errorf("open apphost service: %w", err)
	}
	defer service.Close()
	query, err := service.Query()
	if err != nil {
		return "", fmt.Errorf("query apphost service: %w", err)
	}
	cfg, err := service.Config()
	if err != nil {
		return serviceStateName(query.State), nil
	}
	return fmt.Sprintf("%s start_type=%s delayed_auto_start=%t pid=%d", serviceStateName(query.State), serviceStartTypeName(cfg.StartType), cfg.DelayedAutoStart, query.ProcessId), nil
}

func waitForServiceState(ctx context.Context, service *mgr.Service, want svc.State, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var last svc.State
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		query, err := service.Query()
		if err != nil {
			return fmt.Errorf("query apphost service: %w", err)
		}
		last = query.State
		if query.State == want {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("apphost service state is %s, want %s", serviceStateName(last), serviceStateName(want))
}

func serviceStateName(state svc.State) string {
	switch state {
	case svc.Stopped:
		return "stopped"
	case svc.StartPending:
		return "start_pending"
	case svc.StopPending:
		return "stop_pending"
	case svc.Running:
		return "running"
	case svc.ContinuePending:
		return "continue_pending"
	case svc.PausePending:
		return "pause_pending"
	case svc.Paused:
		return "paused"
	default:
		return fmt.Sprintf("unknown(%d)", state)
	}
}

func serviceStartTypeName(startType uint32) string {
	switch startType {
	case mgr.StartAutomatic:
		return "automatic"
	case mgr.StartManual:
		return "manual"
	case mgr.StartDisabled:
		return "disabled"
	default:
		return fmt.Sprintf("unknown(%d)", startType)
	}
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
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		return false, err
	}
	defer token.Close()
	return token.IsMember(adminSID)
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
