package certstore

import (
	"context"
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"
)

type fakePlatform struct {
	name         string
	installed    bool
	installPath  string
	installCalls int
	uninstalled  string
}

func (p *fakePlatform) Name() string {
	if p.name == "" {
		return "windows"
	}
	return p.name
}

func (p *fakePlatform) IsInstalled(context.Context, *x509.Certificate, string) (bool, error) {
	return p.installed, nil
}

func (p *fakePlatform) Install(_ context.Context, _ *x509.Certificate, certPath string) error {
	p.installed = true
	p.installPath = certPath
	p.installCalls++
	return nil
}

func (p *fakePlatform) Uninstall(_ context.Context, cert *x509.Certificate) error {
	p.installed = false
	p.uninstalled = Thumbprint(cert)
	return nil
}

func TestRootCAIsGeneratedAndReused(t *testing.T) {
	dir := t.TempDir()
	manager := NewWithPlatform(Config{Dir: dir}, &fakePlatform{})
	root, err := manager.EnsureRootCA()
	if err != nil {
		t.Fatal(err)
	}
	if !root.IsCA {
		t.Fatalf("root certificate is not a CA")
	}
	if _, err := os.Stat(filepath.Join(dir, rootCertFile)); err != nil {
		t.Fatalf("root cert not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, rootKeyFile)); err != nil {
		t.Fatalf("root key not written: %v", err)
	}

	manager2 := NewWithPlatform(Config{Dir: dir}, &fakePlatform{})
	root2, err := manager2.EnsureRootCA()
	if err != nil {
		t.Fatal(err)
	}
	if Thumbprint(root) != Thumbprint(root2) {
		t.Fatalf("root CA was not reused")
	}
}

func TestDynamicCertificateHasSANAndCaches(t *testing.T) {
	manager := NewWithPlatform(Config{Dir: t.TempDir()}, &fakePlatform{})
	cert1, err := manager.Certificate("STORE.STEAMPOWERED.COM")
	if err != nil {
		t.Fatal(err)
	}
	cert2, err := manager.Certificate("store.steampowered.com")
	if err != nil {
		t.Fatal(err)
	}
	if cert1 != cert2 {
		t.Fatalf("certificate cache was not used")
	}
	leaf := cert1.Leaf
	if leaf == nil {
		t.Fatalf("leaf certificate is nil")
	}
	if err := leaf.VerifyHostname("store.steampowered.com"); err != nil {
		t.Fatalf("hostname verification failed: %v", err)
	}
}

func TestInstallAndUninstallUsePlatform(t *testing.T) {
	platform := &fakePlatform{}
	manager := NewWithPlatform(Config{Dir: t.TempDir()}, platform)
	if err := manager.Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !platform.installed || platform.installPath == "" {
		t.Fatalf("install did not use platform: %#v", platform)
	}
	if platform.installCalls != 1 {
		t.Fatalf("install calls = %d, want 1", platform.installCalls)
	}
	if err := manager.Uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	if platform.installed || platform.uninstalled == "" {
		t.Fatalf("uninstall did not use platform: %#v", platform)
	}
}

func TestInstallSkipsWhenAlreadyInstalled(t *testing.T) {
	platform := &fakePlatform{installed: true}
	manager := NewWithPlatform(Config{Dir: t.TempDir()}, platform)
	if err := manager.Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	if platform.installCalls != 0 {
		t.Fatalf("install calls = %d, want 0", platform.installCalls)
	}
}
