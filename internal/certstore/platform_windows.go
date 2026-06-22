//go:build windows

package certstore

import (
	"context"
	"crypto/sha1"
	"crypto/x509"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"unsafe"

	"github.com/gofurry/go-steam-core/internal/config"
	"golang.org/x/sys/windows"
)

const cryptENotFound windows.Errno = 0x80092004

type windowsPlatform struct{}

func newOSPlatform() Platform {
	return windowsPlatform{}
}

func (p windowsPlatform) Name() string {
	return runtime.GOOS
}

func (p windowsPlatform) IsInstalled(_ context.Context, cert *x509.Certificate, _ string, storeScope string) (bool, error) {
	store, err := openRootStore(storeScope, false)
	if err != nil {
		return false, err
	}
	defer windows.CertCloseStore(store, 0)
	ctx, err := findCertBySHA1(store, cert)
	if err != nil {
		if errors.Is(err, cryptENotFound) {
			return false, nil
		}
		return false, err
	}
	defer windows.CertFreeCertificateContext(ctx)
	return true, nil
}

func (p windowsPlatform) Install(_ context.Context, cert *x509.Certificate, certPath string, storeScope string) error {
	store, err := openRootStore(storeScope, true)
	if err != nil {
		return err
	}
	defer windows.CertCloseStore(store, 0)

	ctx, err := certificateContext(cert)
	if err != nil {
		return err
	}
	defer windows.CertFreeCertificateContext(ctx)

	var added *windows.CertContext
	if err := windows.CertAddCertificateContextToStore(store, ctx, windows.CERT_STORE_ADD_REPLACE_EXISTING, &added); err != nil {
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) && isMachineStore(storeScope) {
			return fmt.Errorf("install root CA in %s Root store with Windows certificate store API: %w; rerun as Administrator or set cert.store_scope: user", windowsStoreLabel(storeScope), err)
		}
		return fmt.Errorf("install root CA in %s Root store with Windows certificate store API: %w", windowsStoreLabel(storeScope), err)
	}
	if added != nil {
		_ = windows.CertFreeCertificateContext(added)
	}
	_ = certPath
	return nil
}

func (p windowsPlatform) Uninstall(_ context.Context, cert *x509.Certificate, storeScope string) error {
	store, err := openRootStore(storeScope, true)
	if err != nil {
		return err
	}
	defer windows.CertCloseStore(store, 0)

	ctx, err := findCertBySHA1(store, cert)
	if err != nil {
		if errors.Is(err, cryptENotFound) {
			return nil
		}
		return err
	}
	if err := windows.CertDeleteCertificateFromStore(ctx); err != nil {
		return fmt.Errorf("uninstall root CA from %s Root store with Windows certificate store API: %w", windowsStoreLabel(storeScope), err)
	}
	return nil
}

func openRootStore(storeScope string, writable bool) (windows.Handle, error) {
	name, err := windows.UTF16PtrFromString("Root")
	if err != nil {
		return 0, err
	}
	flags, err := windowsRootStoreOpenFlags(storeScope, writable)
	if err != nil {
		return 0, err
	}
	store, err := windows.CertOpenStore(
		uintptr(windows.CERT_STORE_PROV_SYSTEM),
		windows.X509_ASN_ENCODING|windows.PKCS_7_ASN_ENCODING,
		0,
		flags,
		uintptr(unsafe.Pointer(name)),
	)
	if err != nil {
		return 0, fmt.Errorf("open %s Root certificate store: %w", windowsStoreLabel(storeScope), err)
	}
	return store, nil
}

func windowsRootStoreOpenFlags(storeScope string, writable bool) (uint32, error) {
	location, err := windowsStoreLocation(storeScope)
	if err != nil {
		return 0, err
	}
	flags := location | windows.CERT_STORE_OPEN_EXISTING_FLAG
	if writable {
		flags |= windows.CERT_STORE_MAXIMUM_ALLOWED_FLAG
	} else {
		flags |= windows.CERT_STORE_READONLY_FLAG
	}
	return flags, nil
}

func windowsStoreLocation(storeScope string) (uint32, error) {
	switch strings.ToLower(strings.TrimSpace(storeScope)) {
	case "", config.CertStoreMachine, "local_machine", "local-machine":
		return windows.CERT_SYSTEM_STORE_LOCAL_MACHINE, nil
	case config.CertStoreUser, "current_user", "current-user":
		return windows.CERT_SYSTEM_STORE_CURRENT_USER, nil
	default:
		return 0, fmt.Errorf("unsupported Windows cert store scope %q", storeScope)
	}
}

func windowsStoreLabel(storeScope string) string {
	switch strings.ToLower(strings.TrimSpace(storeScope)) {
	case "", config.CertStoreMachine, "local_machine", "local-machine":
		return "LocalMachine"
	case config.CertStoreUser, "current_user", "current-user":
		return "CurrentUser"
	default:
		return storeScope
	}
}

func isMachineStore(storeScope string) bool {
	switch strings.ToLower(strings.TrimSpace(storeScope)) {
	case "", config.CertStoreMachine, "local_machine", "local-machine":
		return true
	default:
		return false
	}
}

func certificateContext(cert *x509.Certificate) (*windows.CertContext, error) {
	if len(cert.Raw) == 0 {
		return nil, fmt.Errorf("certificate DER is empty")
	}
	ctx, err := windows.CertCreateCertificateContext(
		windows.X509_ASN_ENCODING|windows.PKCS_7_ASN_ENCODING,
		&cert.Raw[0],
		uint32(len(cert.Raw)),
	)
	if err != nil {
		return nil, fmt.Errorf("create certificate context: %w", err)
	}
	return ctx, nil
}

func findCertBySHA1(store windows.Handle, cert *x509.Certificate) (*windows.CertContext, error) {
	sum := sha1.Sum(cert.Raw)
	blob := windows.CryptHashBlob{
		Size: uint32(len(sum)),
		Data: &sum[0],
	}
	ctx, err := windows.CertFindCertificateInStore(
		store,
		windows.X509_ASN_ENCODING|windows.PKCS_7_ASN_ENCODING,
		0,
		windows.CERT_FIND_SHA1_HASH,
		unsafe.Pointer(&blob),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("find root CA by thumbprint: %w", err)
	}
	return ctx, nil
}
