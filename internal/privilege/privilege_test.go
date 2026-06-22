package privilege

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofurry/go-steam-core/internal/config"
	"github.com/gofurry/go-steam-core/internal/hosts"
)

func TestValidateHelperRequestAcceptsPrepareHostsStart(t *testing.T) {
	req := validPrepareRequest()
	if err := validateHelperRequest(req, "token", 1234); err != nil {
		t.Fatalf("validateHelperRequest() error = %v", err)
	}
}

func TestValidateHelperRequestRejectsInvalidToken(t *testing.T) {
	req := validPrepareRequest()
	req.Token = "other"
	err := validateHelperRequest(req, "token", 1234)
	if err == nil || !strings.Contains(err.Error(), "invalid helper token") {
		t.Fatalf("validateHelperRequest() error = %v, want invalid helper token", err)
	}
}

func TestValidateHelperRequestRejectsUnsupportedCommand(t *testing.T) {
	req := validPrepareRequest()
	req.Command = "shell"
	err := validateHelperRequest(req, "token", 1234)
	if err == nil || !strings.Contains(err.Error(), "unsupported helper command") {
		t.Fatalf("validateHelperRequest() error = %v, want unsupported helper command", err)
	}
}

func TestValidateHelperRequestRejectsCustomHostsPath(t *testing.T) {
	req := validPrepareRequest()
	req.Hosts.Path = filepath.Join(t.TempDir(), "hosts")
	err := validateHelperRequest(req, "token", 1234)
	if err == nil || !strings.Contains(err.Error(), "default Windows hosts file") {
		t.Fatalf("validateHelperRequest() error = %v, want default hosts path rejection", err)
	}
}

func TestValidateHelperRequestRejectsRollbackOutsideRuntimeDir(t *testing.T) {
	req := validPrepareRequest()
	req.Hosts.RollbackPath = filepath.Join(t.TempDir(), "rollback.json")
	err := validateHelperRequest(req, "token", 1234)
	if err == nil || !strings.Contains(err.Error(), "outside the allowed project runtime directory") {
		t.Fatalf("validateHelperRequest() error = %v, want runtime dir rejection", err)
	}
}

func TestValidateHelperRequestRejectsCertDirOutsideConfigDir(t *testing.T) {
	req := validPrepareRequest()
	req.Cert.Dir = filepath.Join(t.TempDir(), "certs")
	err := validateHelperRequest(req, "token", 1234)
	if err == nil || !strings.Contains(err.Error(), "outside the allowed project config directory") {
		t.Fatalf("validateHelperRequest() error = %v, want config dir rejection", err)
	}
}

func TestValidateHelperRequestAcceptsProjectSuffixCertDir(t *testing.T) {
	req := validPrepareRequest()
	req.Cert.Dir = filepath.Join(t.TempDir(), "user", "AppData", "Roaming", "steam-accelerator-core", "certs")
	if err := validateHelperRequest(req, "token", 1234); err != nil {
		t.Fatalf("validateHelperRequest() error = %v", err)
	}
}

func TestValidateHelperRequestRejectsUnsupportedCertStoreScope(t *testing.T) {
	req := validPrepareRequest()
	req.Cert.StoreScope = "private"
	err := validateHelperRequest(req, "token", 1234)
	if err == nil || !strings.Contains(err.Error(), "unsupported cert store scope") {
		t.Fatalf("validateHelperRequest() error = %v, want store scope rejection", err)
	}
}

func TestValidateHelperRequestAcceptsRestoreHosts(t *testing.T) {
	req := HelperRequest{
		Version:      helperVersion,
		Token:        "token",
		ParentPID:    1234,
		Command:      CommandRestoreHosts,
		RollbackPath: config.DefaultRollbackPath(),
	}
	if err := validateHelperRequest(req, "token", 1234); err != nil {
		t.Fatalf("validateHelperRequest() error = %v", err)
	}
}

func TestValidateHelperRequestAcceptsProjectSuffixRollbackPath(t *testing.T) {
	req := HelperRequest{
		Version:      helperVersion,
		Token:        "token",
		ParentPID:    1234,
		Command:      CommandRestoreHosts,
		RollbackPath: filepath.Join(t.TempDir(), "user", "AppData", "Local", "steam-accelerator-core", "rollback.json"),
	}
	if err := validateHelperRequest(req, "token", 1234); err != nil {
		t.Fatalf("validateHelperRequest() error = %v", err)
	}
}

func validPrepareRequest() HelperRequest {
	return HelperRequest{
		Version:   helperVersion,
		Token:     "token",
		ParentPID: 1234,
		Command:   CommandPrepareHostsStart,
		Cert: CertRequest{
			Dir:        config.DefaultCertDir(),
			StoreScope: config.CertStoreMachine,
		},
		Hosts: hosts.Config{
			Path:         config.DefaultHostsPath(),
			RollbackPath: config.DefaultRollbackPath(),
			MapIP:        "127.0.0.1",
			Entries: []hosts.Entry{{
				IP:   "127.0.0.1",
				Host: "steamcommunity.com",
			}},
		},
		AutoInstall: true,
	}
}
