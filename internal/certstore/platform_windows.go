//go:build windows

package certstore

import (
	"context"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf16"
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

func (p windowsPlatform) Install(opCtx context.Context, cert *x509.Certificate, certPath string, storeScope string) error {
	store, err := openRootStore(storeScope, true)
	if err != nil {
		return installRootWithFallbacks(opCtx, certPath, storeScope, err)
	}
	defer windows.CertCloseStore(store, 0)

	certCtx, err := certificateContext(cert)
	if err != nil {
		return err
	}
	defer windows.CertFreeCertificateContext(certCtx)

	if err := windows.CertAddCertificateContextToStore(store, certCtx, windows.CERT_STORE_ADD_REPLACE_EXISTING_INHERIT_PROPERTIES, nil); err != nil {
		return installRootWithFallbacks(opCtx, certPath, storeScope, fmt.Errorf("install root CA in %s Root store with Windows certificate store API: %w", windowsStoreLabel(storeScope), err))
	}
	_ = certPath
	return nil
}

func (p windowsPlatform) Uninstall(opCtx context.Context, cert *x509.Certificate, storeScope string) error {
	store, err := openRootStore(storeScope, true)
	if err != nil {
		if fallbackErr := uninstallRootWithDotNetX509Store(opCtx, Thumbprint(cert), storeScope); fallbackErr == nil {
			return nil
		} else {
			return fmt.Errorf("%w; .NET X509Store fallback failed: %v", err, fallbackErr)
		}
	}
	defer windows.CertCloseStore(store, 0)

	certCtx, err := findCertBySHA1(store, cert)
	if err != nil {
		if errors.Is(err, cryptENotFound) {
			return nil
		}
		return err
	}
	if err := windows.CertDeleteCertificateFromStore(certCtx); err != nil {
		return uninstallRootWithFallbacks(opCtx, Thumbprint(cert), storeScope, fmt.Errorf("uninstall root CA from %s Root store with Windows certificate store API: %w", windowsStoreLabel(storeScope), err))
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
		flags = location
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

func installRootWithFallbacks(ctx context.Context, certPath string, storeScope string, cause error) error {
	if fallbackErr := installRootWithDotNetX509Store(ctx, certPath, storeScope); fallbackErr == nil {
		return nil
	} else if isMachineStore(storeScope) {
		if userErr := installRootWithDotNetX509Store(ctx, certPath, config.CertStoreUser); userErr == nil {
			return nil
		} else if errors.Is(cause, windows.ERROR_ACCESS_DENIED) {
			return fmt.Errorf("%w; .NET X509Store fallback failed: %v; CurrentUser Root fallback failed: %v; rerun as Administrator or set cert.store_scope: user", cause, fallbackErr, userErr)
		} else {
			return fmt.Errorf("%w; .NET X509Store fallback failed: %v; CurrentUser Root fallback failed: %v", cause, fallbackErr, userErr)
		}
	} else {
		return fmt.Errorf("%w; .NET X509Store fallback failed: %v", cause, fallbackErr)
	}
}

func uninstallRootWithFallbacks(ctx context.Context, thumbprint string, storeScope string, cause error) error {
	if fallbackErr := uninstallRootWithDotNetX509Store(ctx, thumbprint, storeScope); fallbackErr == nil {
		return nil
	} else if isMachineStore(storeScope) {
		if userErr := uninstallRootWithDotNetX509Store(ctx, thumbprint, config.CertStoreUser); userErr == nil {
			return nil
		} else {
			return fmt.Errorf("%w; .NET X509Store fallback failed: %v; CurrentUser Root fallback failed: %v", cause, fallbackErr, userErr)
		}
	} else {
		return fmt.Errorf("%w; .NET X509Store fallback failed: %v", cause, fallbackErr)
	}
}

func installRootWithDotNetX509Store(ctx context.Context, certPath string, storeScope string) error {
	if strings.TrimSpace(certPath) == "" {
		return fmt.Errorf("certificate path is empty")
	}
	absCertPath, err := filepath.Abs(certPath)
	if err != nil {
		return fmt.Errorf("resolve certificate path: %w", err)
	}
	const script = `
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$certPath = $env:SITEBOOST_CERT_PATH
$locationName = $env:SITEBOOST_CERT_STORE_LOCATION
if ([string]::IsNullOrWhiteSpace($certPath)) { throw 'SITEBOOST_CERT_PATH is empty' }
if ([string]::IsNullOrWhiteSpace($locationName)) { throw 'SITEBOOST_CERT_STORE_LOCATION is empty' }
$location = [Enum]::Parse([System.Security.Cryptography.X509Certificates.StoreLocation], $locationName)
$cert = [System.Security.Cryptography.X509Certificates.X509Certificate2]::new($certPath)
$store = [System.Security.Cryptography.X509Certificates.X509Store]::new([System.Security.Cryptography.X509Certificates.StoreName]::Root, $location)
try {
    $store.Open([System.Security.Cryptography.X509Certificates.OpenFlags]::ReadWrite)
    $found = $store.Certificates.Find([System.Security.Cryptography.X509Certificates.X509FindType]::FindByThumbprint, $cert.Thumbprint, $false)
    if ($found.Count -eq 0) {
        $store.Add($cert)
    }
}
finally {
    if ($store -ne $null) { $store.Close() }
    if ($cert -ne $null) { $cert.Dispose() }
}
`
	return runPowerShellEncoded(ctx, script,
		"SITEBOOST_CERT_PATH="+absCertPath,
		"SITEBOOST_CERT_STORE_LOCATION="+windowsStoreLabel(storeScope),
	)
}

func uninstallRootWithDotNetX509Store(ctx context.Context, thumbprint string, storeScope string) error {
	if strings.TrimSpace(thumbprint) == "" {
		return fmt.Errorf("certificate thumbprint is empty")
	}
	const script = `
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$thumbprint = $env:SITEBOOST_CERT_THUMBPRINT
$locationName = $env:SITEBOOST_CERT_STORE_LOCATION
if ([string]::IsNullOrWhiteSpace($thumbprint)) { throw 'SITEBOOST_CERT_THUMBPRINT is empty' }
if ([string]::IsNullOrWhiteSpace($locationName)) { throw 'SITEBOOST_CERT_STORE_LOCATION is empty' }
$location = [Enum]::Parse([System.Security.Cryptography.X509Certificates.StoreLocation], $locationName)
$store = [System.Security.Cryptography.X509Certificates.X509Store]::new([System.Security.Cryptography.X509Certificates.StoreName]::Root, $location)
try {
    $store.Open([System.Security.Cryptography.X509Certificates.OpenFlags]::ReadWrite)
    $found = $store.Certificates.Find([System.Security.Cryptography.X509Certificates.X509FindType]::FindByThumbprint, $thumbprint, $false)
    foreach ($cert in $found) {
        $store.Remove($cert)
    }
}
finally {
    if ($store -ne $null) { $store.Close() }
}
`
	return runPowerShellEncoded(ctx, script,
		"SITEBOOST_CERT_THUMBPRINT="+thumbprint,
		"SITEBOOST_CERT_STORE_LOCATION="+windowsStoreLabel(storeScope),
	)
}

func runPowerShellEncoded(ctx context.Context, script string, extraEnv ...string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, "powershell.exe",
		"-NoLogo",
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-EncodedCommand", powerShellEncodedCommand(script),
	)
	cmd.Env = append(os.Environ(), extraEnv...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	text := strings.TrimSpace(string(out))
	if text != "" {
		return fmt.Errorf("%w: %s", err, text)
	}
	return err
}

func powerShellEncodedCommand(script string) string {
	encoded := utf16.Encode([]rune(script))
	buf := make([]byte, len(encoded)*2)
	for i, r := range encoded {
		binary.LittleEndian.PutUint16(buf[i*2:], r)
	}
	return base64.StdEncoding.EncodeToString(buf)
}
