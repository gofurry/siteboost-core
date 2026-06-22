package privilege

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gofurry/go-steam-core/internal/certstore"
	"github.com/gofurry/go-steam-core/internal/config"
	"github.com/gofurry/go-steam-core/internal/hosts"
)

const (
	helperVersion = 1

	CommandPrepareHostsStart = "prepare-hosts-start"
	CommandTrustRootCA       = "trust-root-ca"
	CommandUntrustRootCA     = "untrust-root-ca"
	CommandRestoreHosts      = "restore-hosts"
	CommandAppHostHealth     = "apphost-health"

	defaultHelperTimeout = 2 * time.Minute
)

var errHelperNotAvailable = errors.New("elevated helper is not available")

type CertRequest struct {
	Dir        string `json:"dir"`
	StoreScope string `json:"store_scope"`
}

type HelperRequest struct {
	Version      int          `json:"version"`
	Token        string       `json:"token"`
	ParentPID    int          `json:"parent_pid"`
	Command      string       `json:"command"`
	Cert         CertRequest  `json:"cert,omitempty"`
	Hosts        hosts.Config `json:"hosts,omitempty"`
	RollbackPath string       `json:"rollback_path,omitempty"`
	AutoInstall  bool         `json:"auto_install,omitempty"`
}

type PrepareHostsResult struct {
	Cert        certstore.TrustResult `json:"cert"`
	CertTrusted bool                  `json:"cert_trusted"`
	Entries     int                   `json:"entries"`
}

type HelperResponse struct {
	OK      bool                   `json:"ok"`
	Error   string                 `json:"error,omitempty"`
	Trust   *certstore.TrustResult `json:"trust,omitempty"`
	Prepare *PrepareHostsResult    `json:"prepare,omitempty"`
}

func ShouldUseHelper() bool {
	return runtime.GOOS == "windows" && !HasSystemPrivileges()
}

func PrepareHostsStart(ctx context.Context, certCfg certstore.Config, hostsCfg hosts.Config, autoInstall bool) (PrepareHostsResult, error) {
	if !ShouldUseHelper() {
		return prepareHostsStartDirect(ctx, certCfg, hostsCfg, autoInstall)
	}
	return PrepareHostsStartElevated(ctx, certCfg, hostsCfg, autoInstall)
}

func PrepareHostsStartElevated(ctx context.Context, certCfg certstore.Config, hostsCfg hosts.Config, autoInstall bool) (PrepareHostsResult, error) {
	resp, err := runPrivilegedRequest(ctx, HelperRequest{
		Command:     CommandPrepareHostsStart,
		Cert:        certRequestFromConfig(certCfg),
		Hosts:       hostsCfg,
		AutoInstall: autoInstall,
	})
	if err != nil {
		return PrepareHostsResult{}, err
	}
	if resp.Prepare == nil {
		return PrepareHostsResult{}, fmt.Errorf("elevated helper did not return prepare result")
	}
	resp.Prepare.Cert.ViaHelper = true
	return *resp.Prepare, nil
}

func EnsureCertTrusted(ctx context.Context, cfg certstore.Config) (certstore.TrustResult, error) {
	if shouldUseCertHelper(cfg) {
		return EnsureCertTrustedElevated(ctx, cfg)
	}
	trust, err := certstore.New(cfg).EnsureTrusted(ctx)
	if err != nil && shouldRetryCertWithHelper(err, cfg) {
		return EnsureCertTrustedElevated(ctx, cfg)
	}
	return trust, err
}

func EnsureCertTrustedElevated(ctx context.Context, cfg certstore.Config) (certstore.TrustResult, error) {
	resp, err := runPrivilegedRequest(ctx, HelperRequest{
		Command: CommandTrustRootCA,
		Cert:    certRequestFromConfig(cfg),
	})
	if err != nil {
		return certstore.TrustResult{}, err
	}
	if resp.Trust == nil {
		return certstore.TrustResult{}, fmt.Errorf("elevated helper did not return certificate trust result")
	}
	resp.Trust.ViaHelper = true
	return *resp.Trust, nil
}

func InstallCert(ctx context.Context, cfg certstore.Config) error {
	_, err := EnsureCertTrusted(ctx, cfg)
	return err
}

func UninstallCert(ctx context.Context, cfg certstore.Config) error {
	if shouldUseCertHelper(cfg) {
		return UninstallCertElevated(ctx, cfg)
	}
	err := certstore.New(cfg).Uninstall(ctx)
	if err != nil && shouldRetryCertWithHelper(err, cfg) {
		return UninstallCertElevated(ctx, cfg)
	}
	return err
}

func UninstallCertElevated(ctx context.Context, cfg certstore.Config) error {
	_, err := runPrivilegedRequest(ctx, HelperRequest{
		Command: CommandUntrustRootCA,
		Cert:    certRequestFromConfig(cfg),
	})
	return err
}

func RestoreHosts(ctx context.Context, rollbackPath string) error {
	if ShouldUseHelper() {
		return RestoreHostsElevated(ctx, rollbackPath)
	}
	err := hosts.Restore(ctx, rollbackPath)
	if err != nil && shouldRetrySystemChangeWithHelper(err) {
		return RestoreHostsElevated(ctx, rollbackPath)
	}
	return err
}

func RestoreHostsElevated(ctx context.Context, rollbackPath string) error {
	_, err := runPrivilegedRequest(ctx, HelperRequest{
		Command:      CommandRestoreHosts,
		RollbackPath: rollbackPath,
	})
	return err
}

func runPrivilegedRequest(ctx context.Context, req HelperRequest) (HelperResponse, error) {
	if runtime.GOOS == "windows" {
		return runAppHostRequest(ctx, req)
	}
	return runElevatedHelper(ctx, req)
}

func runAppHostRequest(ctx context.Context, req HelperRequest) (HelperResponse, error) {
	return runAppHostRequestPlatform(ctx, req)
}

func RunHelper(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("helper", flag.ContinueOnError)
	fs.SetOutput(stderr)
	requestPath := fs.String("request", "", "helper request JSON path")
	responsePath := fs.String("response", "", "helper response JSON path")
	token := fs.String("token", "", "helper request token")
	parentPID := fs.Int("parent", 0, "parent process id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_ = stdout
	if strings.TrimSpace(*requestPath) == "" || strings.TrimSpace(*responsePath) == "" {
		return fmt.Errorf("helper request and response paths are required")
	}
	req, err := readHelperRequest(*requestPath)
	if err != nil {
		_ = writeHelperResponse(*responsePath, HelperResponse{OK: false, Error: err.Error()})
		return err
	}
	if err := validateHelperRequest(req, *token, *parentPID); err != nil {
		_ = writeHelperResponse(*responsePath, HelperResponse{OK: false, Error: err.Error()})
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultHelperTimeout)
	defer cancel()
	resp, err := executeHelperRequest(ctx, req)
	if err != nil {
		resp = HelperResponse{OK: false, Error: fmt.Sprintf("%s: %v", helperStatus(), err)}
	}
	if writeErr := writeHelperResponse(*responsePath, resp); writeErr != nil && err == nil {
		err = writeErr
	}
	return err
}

func executeHelperRequest(ctx context.Context, req HelperRequest) (HelperResponse, error) {
	if runtime.GOOS == "windows" && !HasSystemPrivileges() {
		return HelperResponse{}, fmt.Errorf("elevated helper did not receive an administrator token: %s", helperStatus())
	}
	switch req.Command {
	case CommandAppHostHealth:
		return HelperResponse{OK: true}, nil
	case CommandPrepareHostsStart:
		result, err := prepareHostsStartDirect(ctx, certConfigFromRequest(req.Cert), req.Hosts, req.AutoInstall)
		if err != nil {
			return HelperResponse{}, err
		}
		result.Cert.ViaHelper = true
		return HelperResponse{OK: true, Prepare: &result}, nil
	case CommandTrustRootCA:
		trust, err := certstore.New(certConfigFromRequest(req.Cert)).EnsureTrusted(ctx)
		if err != nil {
			return HelperResponse{}, err
		}
		trust.ViaHelper = true
		return HelperResponse{OK: true, Trust: &trust}, nil
	case CommandUntrustRootCA:
		if err := certstore.New(certConfigFromRequest(req.Cert)).Uninstall(ctx); err != nil {
			return HelperResponse{}, err
		}
		return HelperResponse{OK: true}, nil
	case CommandRestoreHosts:
		if err := hosts.Restore(ctx, req.RollbackPath); err != nil {
			return HelperResponse{}, err
		}
		return HelperResponse{OK: true}, nil
	default:
		return HelperResponse{}, fmt.Errorf("unsupported helper command %q", req.Command)
	}
}

func prepareHostsStartDirect(ctx context.Context, certCfg certstore.Config, hostsCfg hosts.Config, autoInstall bool) (PrepareHostsResult, error) {
	manager := certstore.New(certCfg)
	certInstalled, err := manager.IsInstalled(ctx)
	if err != nil {
		return PrepareHostsResult{}, fmt.Errorf("check root CA install: %w", err)
	}
	var trust certstore.TrustResult
	if certInstalled {
		trust = certstore.TrustResult{
			Platform:       runtime.GOOS,
			CertPath:       manager.RootCertPath(),
			StoreScope:     certCfg.StoreScope,
			AlreadyTrusted: true,
		}
	} else {
		if !autoInstall {
			return PrepareHostsResult{}, fmt.Errorf("local root CA is not installed; run `steam-accelerator cert install` first or enable cert.auto_install")
		}
		trust, err = manager.EnsureTrusted(ctx)
		if err != nil {
			return PrepareHostsResult{}, fmt.Errorf("install local root CA: %w", err)
		}
	}
	if err := hosts.Preflight(ctx, hostsCfg); err != nil {
		return PrepareHostsResult{}, fmt.Errorf("preflight hosts mode: %w", err)
	}
	if err := hosts.Apply(ctx, hostsCfg); err != nil {
		return PrepareHostsResult{}, fmt.Errorf("apply hosts: %w", err)
	}
	return PrepareHostsResult{Cert: trust, CertTrusted: true, Entries: len(hostsCfg.Entries)}, nil
}

func validateHelperRequest(req HelperRequest, token string, parentPID int) error {
	if req.Version != helperVersion {
		return fmt.Errorf("unsupported helper request version %d", req.Version)
	}
	if strings.TrimSpace(token) == "" || req.Token != token {
		return fmt.Errorf("invalid helper token")
	}
	if parentPID <= 0 || req.ParentPID != parentPID {
		return fmt.Errorf("invalid helper parent pid")
	}
	return validatePrivilegedRequest(req)
}

func validateAppHostRequest(req HelperRequest, clientPID int) error {
	if req.Version != helperVersion {
		return fmt.Errorf("unsupported apphost request version %d", req.Version)
	}
	if req.ParentPID <= 0 {
		return fmt.Errorf("invalid apphost parent pid")
	}
	if strings.TrimSpace(req.Token) == "" {
		return fmt.Errorf("invalid apphost token")
	}
	if clientPID <= 0 {
		return fmt.Errorf("invalid apphost client pid")
	}
	if req.ParentPID != clientPID {
		return fmt.Errorf("apphost client pid mismatch: request parent=%d pipe client=%d", req.ParentPID, clientPID)
	}
	return validatePrivilegedRequest(req)
}

func validatePrivilegedRequest(req HelperRequest) error {
	switch req.Command {
	case CommandAppHostHealth:
		return nil
	case CommandPrepareHostsStart:
		if err := validateCertRequest(req.Cert); err != nil {
			return err
		}
		return validateHostsConfigForHelper(req.Hosts)
	case CommandTrustRootCA, CommandUntrustRootCA:
		return validateCertRequest(req.Cert)
	case CommandRestoreHosts:
		return validateRollbackPath(req.RollbackPath)
	default:
		return fmt.Errorf("unsupported helper command %q", req.Command)
	}
}

func validateCertRequest(req CertRequest) error {
	cfg := certConfigFromRequest(req)
	if strings.TrimSpace(cfg.Dir) == "" {
		return fmt.Errorf("cert dir is required")
	}
	switch strings.ToLower(strings.TrimSpace(cfg.StoreScope)) {
	case "", config.CertStoreMachine, config.CertStoreUser:
	default:
		return fmt.Errorf("unsupported cert store scope %q", cfg.StoreScope)
	}
	if !isAllowedProjectPath(cfg.Dir, config.DefaultCertDir()) {
		return fmt.Errorf("cert dir %q is outside the allowed project config directory", cfg.Dir)
	}
	return nil
}

func validateHostsConfigForHelper(cfg hosts.Config) error {
	if !samePath(cfg.Path, config.DefaultHostsPath()) {
		return fmt.Errorf("elevated helper can only modify the default Windows hosts file")
	}
	if err := validateRollbackPath(cfg.RollbackPath); err != nil {
		return err
	}
	if len(cfg.Entries) == 0 {
		return fmt.Errorf("hosts entries are required")
	}
	for _, entry := range cfg.Entries {
		if strings.TrimSpace(entry.IP) == "" || strings.TrimSpace(entry.Host) == "" {
			return fmt.Errorf("hosts entries cannot contain empty IP or host")
		}
	}
	return nil
}

func validateRollbackPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("rollback path is required")
	}
	if !isAllowedProjectPath(path, config.DefaultRollbackPath()) {
		return fmt.Errorf("rollback path %q is outside the allowed project runtime directory", path)
	}
	return nil
}

func certRequestFromConfig(cfg certstore.Config) CertRequest {
	return CertRequest{Dir: cfg.Dir, StoreScope: cfg.StoreScope}
}

func certConfigFromRequest(req CertRequest) certstore.Config {
	return certstore.Config{Dir: req.Dir, StoreScope: req.StoreScope}
}

func shouldUseCertHelper(cfg certstore.Config) bool {
	if runtime.GOOS != "windows" || HasSystemPrivileges() {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(cfg.StoreScope), config.CertStoreMachine) || strings.TrimSpace(cfg.StoreScope) == ""
}

func shouldRetryCertWithHelper(err error, cfg certstore.Config) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(cfg.StoreScope), config.CertStoreMachine) && strings.TrimSpace(cfg.StoreScope) != "" {
		return false
	}
	return shouldRetrySystemChangeWithHelper(err)
}

func shouldRetrySystemChangeWithHelper(err error) bool {
	if err == nil || runtime.GOOS != "windows" {
		return false
	}
	if os.IsPermission(err) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "access is denied") ||
		strings.Contains(msg, "permission denied") ||
		strings.Contains(msg, "administrator")
}

func helperContext(parent context.Context) (context.Context, context.CancelFunc, error) {
	if err := parent.Err(); err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultHelperTimeout)
	return ctx, cancel, nil
}

func samePath(a, b string) bool {
	aa, err := filepath.Abs(a)
	if err != nil {
		aa = a
	}
	bb, err := filepath.Abs(b)
	if err != nil {
		bb = b
	}
	aa = filepath.Clean(aa)
	bb = filepath.Clean(bb)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(aa, bb)
	}
	return aa == bb
}

func isUnderOrEqual(path, base string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absBase, err := filepath.Abs(base)
	if err != nil {
		return false
	}
	absPath = filepath.Clean(absPath)
	absBase = filepath.Clean(absBase)
	if runtime.GOOS == "windows" {
		absPath = strings.ToLower(absPath)
		absBase = strings.ToLower(absBase)
	}
	if absPath == absBase {
		return true
	}
	rel, err := filepath.Rel(absBase, absPath)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func isAllowedProjectPath(path, defaultPath string) bool {
	if isUnderOrEqual(path, filepath.Dir(defaultPath)) {
		return true
	}
	return hasPathSuffix(path, projectPathSuffix(defaultPath))
}

func projectPathSuffix(path string) []string {
	clean := filepath.Clean(path)
	parts := splitPath(clean)
	for i := len(parts) - 1; i >= 0; i-- {
		if strings.EqualFold(parts[i], "steam-accelerator-core") {
			return parts[i:]
		}
	}
	return parts
}

func hasPathSuffix(path string, suffix []string) bool {
	if len(suffix) == 0 {
		return false
	}
	parts := splitPath(filepath.Clean(path))
	if len(parts) < len(suffix) {
		return false
	}
	parts = parts[len(parts)-len(suffix):]
	for i := range suffix {
		if runtime.GOOS == "windows" {
			if !strings.EqualFold(parts[i], suffix[i]) {
				return false
			}
			continue
		}
		if parts[i] != suffix[i] {
			return false
		}
	}
	return true
}

func splitPath(path string) []string {
	vol := filepath.VolumeName(path)
	if vol != "" {
		path = strings.TrimPrefix(path, vol)
	}
	path = strings.Trim(path, string(os.PathSeparator))
	if path == "" {
		return nil
	}
	return strings.Split(path, string(os.PathSeparator))
}
