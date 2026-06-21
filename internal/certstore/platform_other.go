//go:build !windows

package certstore

import (
	"context"
	"crypto/x509"
	"fmt"
	"runtime"
)

type unsupportedPlatform struct{}

func newOSPlatform() Platform {
	return unsupportedPlatform{}
}

func (unsupportedPlatform) Name() string {
	return runtime.GOOS
}

func (p unsupportedPlatform) IsInstalled(context.Context, *x509.Certificate, string, string) (bool, error) {
	return false, fmt.Errorf("%w: %s", ErrUnsupported, p.Name())
}

func (p unsupportedPlatform) Install(context.Context, *x509.Certificate, string, string) error {
	return fmt.Errorf("%w: %s", ErrUnsupported, p.Name())
}

func (p unsupportedPlatform) Uninstall(context.Context, *x509.Certificate, string) error {
	return fmt.Errorf("%w: %s", ErrUnsupported, p.Name())
}
