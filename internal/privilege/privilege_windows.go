//go:build windows

package privilege

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

const (
	appHostServiceName = "SiteBoostCoreAppHost"
	appHostPipeName    = `\\.\pipe\SiteBoostCoreAppHost`
	appHostMaxFrame    = 1 << 20

	errorServiceMarkedForDelete syscall.Errno = 1072
)

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
	pipeCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	req := HelperRequest{
		Version:   helperVersion,
		Token:     "health",
		ParentPID: os.Getpid(),
		Command:   CommandAppHostHealth,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	respBody, err := appHostPipeRoundTrip(pipeCtx, body)
	if err != nil {
		return err
	}
	var resp HelperResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return err
	}
	if !resp.OK {
		return fmt.Errorf("apphost health failed: %s", resp.Error)
	}
	return nil
}

func runAppHostRequestPlatform(ctx context.Context, req HelperRequest) (HelperResponse, error) {
	if err := ensureAppHostStarted(ctx); err != nil {
		return HelperResponse{}, err
	}
	token, err := randomToken()
	if err != nil {
		return HelperResponse{}, err
	}
	req.Version = helperVersion
	req.ParentPID = os.Getpid()
	req.Token = token
	body, err := json.Marshal(req)
	if err != nil {
		return HelperResponse{}, fmt.Errorf("encode apphost request: %w", err)
	}
	respBody, err := appHostPipeRoundTrip(ctx, body)
	if err != nil {
		return HelperResponse{}, err
	}
	var helperResp HelperResponse
	if err := json.Unmarshal(respBody, &helperResp); err != nil {
		return HelperResponse{}, fmt.Errorf("parse apphost response: %w", err)
	}
	if !helperResp.OK {
		if helperResp.Error == "" {
			helperResp.Error = "apphost request failed"
		}
		return helperResp, fmt.Errorf("windows apphost: %s", helperResp.Error)
	}
	return helperResp, nil
}

func appHostPipeRoundTrip(ctx context.Context, body []byte) ([]byte, error) {
	reqCtx, cancel := context.WithTimeout(ctx, defaultHelperTimeout)
	defer cancel()
	handle, err := openAppHostPipe(reqCtx)
	if err != nil {
		return nil, fmt.Errorf("connect windows apphost pipe: %w", err)
	}
	defer windows.CloseHandle(handle)
	if err := writePipeFrame(handle, body); err != nil {
		return nil, fmt.Errorf("write apphost request: %w", err)
	}
	resp, err := readPipeFrame(handle, appHostMaxFrame)
	if err != nil {
		return nil, fmt.Errorf("read apphost response: %w", err)
	}
	return resp, nil
}

func openAppHostPipe(ctx context.Context) (windows.Handle, error) {
	name, err := windows.UTF16PtrFromString(appHostPipeName)
	if err != nil {
		return windows.InvalidHandle, err
	}
	var lastErr error
	for {
		if err := ctx.Err(); err != nil {
			if lastErr != nil {
				return windows.InvalidHandle, fmt.Errorf("%w: %v", err, lastErr)
			}
			return windows.InvalidHandle, err
		}
		handle, err := windows.CreateFile(
			name,
			windows.GENERIC_READ|windows.GENERIC_WRITE,
			0,
			nil,
			windows.OPEN_EXISTING,
			windows.FILE_ATTRIBUTE_NORMAL,
			0,
		)
		if err == nil {
			return handle, nil
		}
		lastErr = err
		if !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) &&
			!errors.Is(err, windows.ERROR_PATH_NOT_FOUND) &&
			!errors.Is(err, windows.ERROR_PIPE_BUSY) {
			return windows.InvalidHandle, err
		}
		select {
		case <-ctx.Done():
			return windows.InvalidHandle, fmt.Errorf("%w: %v", ctx.Err(), lastErr)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func RunAppHostConsole(ctx context.Context, stdout io.Writer) error {
	if !HasSystemPrivileges() {
		return fmt.Errorf("windows apphost requires system privileges: %s", helperStatus())
	}
	server := newAppHostServer(stdout)
	return server.run(ctx)
}

type appHostServer struct {
	stdout io.Writer
}

func newAppHostServer(stdout io.Writer) *appHostServer {
	if stdout == nil {
		stdout = io.Discard
	}
	return &appHostServer{stdout: stdout}
}

func (s *appHostServer) run(ctx context.Context) error {
	fmt.Fprintf(s.stdout, "windows apphost listening on named pipe %s\n", appHostPipeName)
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		pipe, err := createAppHostPipe()
		if err != nil {
			return err
		}
		connected := make(chan error, 1)
		go func() {
			err := windows.ConnectNamedPipe(pipe, nil)
			if err == nil || errors.Is(err, windows.ERROR_PIPE_CONNECTED) {
				connected <- nil
				return
			}
			connected <- err
		}()
		select {
		case <-ctx.Done():
			_ = windows.CloseHandle(pipe)
			select {
			case <-connected:
			case <-time.After(time.Second):
			}
			return nil
		case err := <-connected:
			if err != nil {
				_ = windows.CloseHandle(pipe)
				return fmt.Errorf("connect apphost pipe: %w", err)
			}
			go s.handlePipe(pipe)
		}
	}
}

func createAppHostPipe() (windows.Handle, error) {
	name, err := windows.UTF16PtrFromString(appHostPipeName)
	if err != nil {
		return windows.InvalidHandle, err
	}
	attrs, err := appHostPipeSecurityAttributes()
	if err != nil {
		return windows.InvalidHandle, err
	}
	handle, err := windows.CreateNamedPipe(
		name,
		windows.PIPE_ACCESS_DUPLEX,
		windows.PIPE_TYPE_BYTE|windows.PIPE_READMODE_BYTE|windows.PIPE_WAIT|windows.PIPE_REJECT_REMOTE_CLIENTS,
		windows.PIPE_UNLIMITED_INSTANCES,
		appHostMaxFrame,
		appHostMaxFrame,
		0,
		attrs,
	)
	if err != nil {
		return windows.InvalidHandle, fmt.Errorf("create apphost named pipe: %w", err)
	}
	return handle, nil
}

func appHostPipeSecurityAttributes() (*windows.SecurityAttributes, error) {
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GRGW;;;IU)")
	if err != nil {
		return nil, fmt.Errorf("create apphost pipe security descriptor: %w", err)
	}
	return &windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: sd,
	}, nil
}

func (s *appHostServer) handlePipe(pipe windows.Handle) {
	defer windows.CloseHandle(pipe)
	defer windows.DisconnectNamedPipe(pipe)
	resp := s.handlePipeRequest(pipe)
	body, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(s.stdout, "encode apphost pipe response: %v\n", err)
		return
	}
	if err := writePipeFrame(pipe, body); err != nil {
		fmt.Fprintf(s.stdout, "write apphost pipe response: %v\n", err)
	}
}

func (s *appHostServer) handlePipeRequest(pipe windows.Handle) HelperResponse {
	var clientPID uint32
	if err := windows.GetNamedPipeClientProcessId(pipe, &clientPID); err != nil {
		return HelperResponse{OK: false, Error: fmt.Sprintf("get apphost pipe client pid: %v", err)}
	}
	if err := validateAppHostClientProcess(clientPID); err != nil {
		return HelperResponse{OK: false, Error: err.Error()}
	}
	body, err := readPipeFrame(pipe, appHostMaxFrame)
	if err != nil {
		return HelperResponse{OK: false, Error: fmt.Sprintf("read apphost pipe request: %v", err)}
	}
	var req HelperRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return HelperResponse{OK: false, Error: fmt.Sprintf("parse apphost pipe request: %v", err)}
	}
	if err := validateAppHostRequest(req, int(clientPID)); err != nil {
		return HelperResponse{OK: false, Error: err.Error()}
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultHelperTimeout)
	defer cancel()
	resp, err := executeHelperRequest(ctx, req)
	if err != nil {
		return HelperResponse{OK: false, Error: fmt.Sprintf("%s: %v", helperStatus(), err)}
	}
	return resp
}

func validateAppHostClientProcess(clientPID uint32) error {
	clientPath, err := processImagePath(clientPID)
	if err != nil {
		return fmt.Errorf("query apphost client process: %w", err)
	}
	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("query apphost executable: %w", err)
	}
	clientPath = normalizeExecutablePath(clientPath)
	selfPath = normalizeExecutablePath(selfPath)
	if !strings.EqualFold(clientPath, selfPath) {
		return fmt.Errorf("apphost client executable mismatch: client=%q apphost=%q", clientPath, selfPath)
	}
	return nil
}

func processImagePath(pid uint32) (string, error) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(handle)
	size := uint32(windows.MAX_LONG_PATH)
	buf := make([]uint16, size)
	if err := windows.QueryFullProcessImageName(handle, 0, &buf[0], &size); err != nil {
		return "", err
	}
	return windows.UTF16ToString(buf[:size]), nil
}

func normalizeExecutablePath(path string) string {
	if strings.HasPrefix(path, `\\?\`) {
		path = strings.TrimPrefix(path, `\\?\`)
	}
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	eval, err := filepath.EvalSymlinks(path)
	if err == nil {
		path = eval
	}
	return filepath.Clean(path)
}

func readPipeFrame(handle windows.Handle, max uint32) ([]byte, error) {
	var header [4]byte
	if err := readPipeFull(handle, header[:]); err != nil {
		return nil, err
	}
	size := binary.BigEndian.Uint32(header[:])
	if size == 0 {
		return nil, fmt.Errorf("empty pipe frame")
	}
	if size > max {
		return nil, fmt.Errorf("pipe frame too large: %d > %d", size, max)
	}
	body := make([]byte, size)
	if err := readPipeFull(handle, body); err != nil {
		return nil, err
	}
	return body, nil
}

func writePipeFrame(handle windows.Handle, body []byte) error {
	if len(body) == 0 {
		return fmt.Errorf("empty pipe frame")
	}
	if len(body) > appHostMaxFrame {
		return fmt.Errorf("pipe frame too large: %d > %d", len(body), appHostMaxFrame)
	}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(body)))
	if err := writePipeFull(handle, header[:]); err != nil {
		return err
	}
	return writePipeFull(handle, body)
}

func readPipeFull(handle windows.Handle, buf []byte) error {
	for len(buf) > 0 {
		var done uint32
		if err := windows.ReadFile(handle, buf, &done, nil); err != nil {
			return err
		}
		if done == 0 {
			return io.ErrUnexpectedEOF
		}
		buf = buf[done:]
	}
	return nil
}

func writePipeFull(handle windows.Handle, buf []byte) error {
	for len(buf) > 0 {
		var done uint32
		if err := windows.WriteFile(handle, buf, &done, nil); err != nil {
			return err
		}
		if done == 0 {
			return io.ErrShortWrite
		}
		buf = buf[done:]
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
			if isServiceMarkedForDelete(err) {
				if waitErr := waitForAppHostServiceDeleted(ctx, manager, 20*time.Second); waitErr != nil {
					return fmt.Errorf("wait for deleted apphost service before reinstall: %w", waitErr)
				}
				return createAppHostService(ctx, manager, exe)
			}
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
	if isServiceMarkedForDelete(err) {
		if waitErr := waitForAppHostServiceDeleted(ctx, manager, 20*time.Second); waitErr != nil {
			return fmt.Errorf("wait for deleted apphost service before reinstall: %w", waitErr)
		}
		return createAppHostService(ctx, manager, exe)
	}
	if !errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return fmt.Errorf("open apphost service: %w", err)
	}
	return createAppHostService(ctx, manager, exe)
}

func createAppHostService(ctx context.Context, manager *mgr.Mgr, exe string) error {
	service, err := manager.CreateService(appHostServiceName, exe, newAppHostServiceConfig(""), "__apphost-service")
	if err != nil {
		if isServiceMarkedForDelete(err) {
			if waitErr := waitForAppHostServiceDeleted(ctx, manager, 20*time.Second); waitErr != nil {
				return fmt.Errorf("wait for deleted apphost service before create: %w", waitErr)
			}
			service, err = manager.CreateService(appHostServiceName, exe, newAppHostServiceConfig(""), "__apphost-service")
			if err != nil {
				return fmt.Errorf("create apphost service: %w", err)
			}
		} else {
			return fmt.Errorf("create apphost service: %w", err)
		}
	}
	defer service.Close()
	return StartAppHostService(ctx)
}

func isServiceMarkedForDelete(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, errorServiceMarkedForDelete) || strings.Contains(strings.ToLower(err.Error()), "marked for deletion")
}

func waitForAppHostServiceDeleted(ctx context.Context, manager *mgr.Mgr, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		service, err := manager.OpenService(appHostServiceName)
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return nil
		}
		if err != nil && !isServiceMarkedForDelete(err) {
			return fmt.Errorf("open apphost service while waiting for deletion: %w", err)
		}
		if service != nil {
			_ = service.Close()
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("apphost service is still marked for deletion; close Services.msc or any process holding the service handle and retry")
		}
		time.Sleep(200 * time.Millisecond)
	}
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
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return nil
		}
		if isServiceMarkedForDelete(err) {
			return waitForAppHostServiceDeleted(ctx, manager, 20*time.Second)
		}
		return fmt.Errorf("open apphost service: %w", err)
	}
	if err := service.Delete(); err != nil {
		_ = service.Close()
		return fmt.Errorf("delete apphost service: %w", err)
	}
	_ = service.Close()
	return waitForAppHostServiceDeleted(ctx, manager, 20*time.Second)
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
