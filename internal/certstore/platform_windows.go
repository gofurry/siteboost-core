//go:build windows

package certstore

import (
	"context"
	"crypto/sha1"
	"crypto/x509"
	"errors"
	"fmt"
	"runtime"
	"unsafe"

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

func (p windowsPlatform) IsInstalled(_ context.Context, cert *x509.Certificate, _ string) (bool, error) {
	store, err := openCurrentUserRootStore()
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

func (p windowsPlatform) Install(_ context.Context, cert *x509.Certificate, certPath string) error {
	store, err := openCurrentUserRootStore()
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
		return fmt.Errorf("install root CA with Windows certificate store API: %w", err)
	}
	if added != nil {
		_ = windows.CertFreeCertificateContext(added)
	}
	_ = certPath
	return nil
}

func (p windowsPlatform) Uninstall(_ context.Context, cert *x509.Certificate) error {
	store, err := openCurrentUserRootStore()
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
		return fmt.Errorf("uninstall root CA with Windows certificate store API: %w", err)
	}
	return nil
}

func openCurrentUserRootStore() (windows.Handle, error) {
	name, err := windows.UTF16PtrFromString("Root")
	if err != nil {
		return 0, err
	}
	store, err := windows.CertOpenStore(
		uintptr(windows.CERT_STORE_PROV_SYSTEM),
		windows.X509_ASN_ENCODING|windows.PKCS_7_ASN_ENCODING,
		0,
		windows.CERT_SYSTEM_STORE_CURRENT_USER,
		uintptr(unsafe.Pointer(name)),
	)
	if err != nil {
		return 0, fmt.Errorf("open current-user Root certificate store: %w", err)
	}
	return store, nil
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
