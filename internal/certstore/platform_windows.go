//go:build windows

package certstore

import (
	"context"
	"crypto/x509"
	"fmt"
	"os/exec"
	"runtime"
)

type commandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

type windowsPlatform struct {
	runner commandRunner
}

func newOSPlatform() Platform {
	return windowsPlatform{runner: execRunner{}}
}

func NewWindowsPlatformForTest(runner commandRunner) Platform {
	return windowsPlatform{runner: runner}
}

func (p windowsPlatform) Name() string {
	return runtime.GOOS
}

func (p windowsPlatform) IsInstalled(ctx context.Context, cert *x509.Certificate, certPath string) (bool, error) {
	output, err := p.run(ctx, "-user", "-store", "Root", Thumbprint(cert))
	if err != nil {
		return false, nil
	}
	_ = output
	return true, nil
}

func (p windowsPlatform) Install(ctx context.Context, cert *x509.Certificate, certPath string) error {
	output, err := p.run(ctx, "-user", "-addstore", "Root", certPath)
	return certutilError("install", output, err)
}

func (p windowsPlatform) Uninstall(ctx context.Context, cert *x509.Certificate) error {
	output, err := p.run(ctx, "-user", "-delstore", "Root", Thumbprint(cert))
	return certutilError("uninstall", output, err)
}

func (p windowsPlatform) run(ctx context.Context, args ...string) ([]byte, error) {
	if p.runner == nil {
		p.runner = execRunner{}
	}
	return p.runner.Run(ctx, "certutil", args...)
}

func certutilError(action string, output []byte, err error) error {
	if err == nil {
		return nil
	}
	if len(output) == 0 {
		return fmt.Errorf("%s root CA with certutil: %w", action, err)
	}
	return fmt.Errorf("%s root CA with certutil: %w: %s", action, err, string(output))
}
