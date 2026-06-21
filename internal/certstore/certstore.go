package certstore

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gofurry/go-steam-core/internal/config"
	"github.com/gofurry/go-steam-core/internal/rules"
)

const (
	rootCertFile = "root-ca.pem"
	rootKeyFile  = "root-ca-key.pem"
	rootName     = "steam-accelerator-core Local Root CA"
)

var (
	ErrNoCA        = errors.New("local root CA is not generated")
	ErrUnsupported = errors.New("certificate store is unsupported on this platform")
)

type Config struct {
	Dir string
}

type Platform interface {
	Name() string
	IsInstalled(ctx context.Context, cert *x509.Certificate, certPath string) (bool, error)
	Install(ctx context.Context, cert *x509.Certificate, certPath string) error
	Uninstall(ctx context.Context, cert *x509.Certificate) error
}

type Manager struct {
	cfg      Config
	platform Platform

	mu       sync.Mutex
	rootCert *x509.Certificate
	rootKey  *ecdsa.PrivateKey
	cache    map[string]*tls.Certificate
	loadedCA bool
}

func ConfigFromApp(cfg config.Config) Config {
	return Config{Dir: cfg.Cert.Dir}
}

func New(cfg Config) *Manager {
	return NewWithPlatform(cfg, newOSPlatform())
}

func NewWithPlatform(cfg Config, platform Platform) *Manager {
	if platform == nil {
		platform = newOSPlatform()
	}
	return &Manager{
		cfg:      cfg,
		platform: platform,
		cache:    make(map[string]*tls.Certificate),
	}
}

func (m *Manager) EnsureRootCA() (*x509.Certificate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ensureRootCALocked()
}

func (m *Manager) RootCertPath() string {
	return filepath.Join(m.cfg.Dir, rootCertFile)
}

func (m *Manager) RootKeyPath() string {
	return filepath.Join(m.cfg.Dir, rootKeyFile)
}

func (m *Manager) IsInstalled(ctx context.Context) (bool, error) {
	if m.platform.Name() != "windows" {
		return false, fmt.Errorf("%w: %s", ErrUnsupported, m.platform.Name())
	}
	cert, err := m.loadRootCA()
	if err != nil {
		if errors.Is(err, ErrNoCA) {
			return false, nil
		}
		return false, err
	}
	return m.platform.IsInstalled(ctx, cert, m.RootCertPath())
}

func (m *Manager) Install(ctx context.Context) error {
	if m.platform.Name() != "windows" {
		return fmt.Errorf("%w: %s", ErrUnsupported, m.platform.Name())
	}
	cert, err := m.EnsureRootCA()
	if err != nil {
		return err
	}
	installed, err := m.platform.IsInstalled(ctx, cert, m.RootCertPath())
	if err != nil {
		return err
	}
	if installed {
		return nil
	}
	return m.platform.Install(ctx, cert, m.RootCertPath())
}

func (m *Manager) Uninstall(ctx context.Context) error {
	if m.platform.Name() != "windows" {
		return fmt.Errorf("%w: %s", ErrUnsupported, m.platform.Name())
	}
	cert, err := m.loadRootCA()
	if err != nil {
		return err
	}
	installed, err := m.platform.IsInstalled(ctx, cert, m.RootCertPath())
	if err != nil {
		return err
	}
	if !installed {
		return nil
	}
	return m.platform.Uninstall(ctx, cert)
}

func (m *Manager) Certificate(host string) (*tls.Certificate, error) {
	normalized, err := rules.NormalizeHost(host)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if cert, ok := m.cache[normalized]; ok {
		return cert, nil
	}
	rootCert, rootKey, err := m.ensureRootKeypairLocked()
	if err != nil {
		return nil, err
	}
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate site private key: %w", err)
	}
	serial, err := randomSerial()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: normalized,
		},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(90 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{normalized},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, rootCert, &leafKey.PublicKey, rootKey)
	if err != nil {
		return nil, fmt.Errorf("create site certificate: %w", err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("parse site certificate: %w", err)
	}
	tlsCert := &tls.Certificate{
		Certificate: [][]byte{der, rootCert.Raw},
		PrivateKey:  leafKey,
		Leaf:        leaf,
	}
	m.cache[normalized] = tlsCert
	return tlsCert, nil
}

func (m *Manager) loadRootCA() (*x509.Certificate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cert, _, err := m.loadRootKeypairLocked()
	return cert, err
}

func (m *Manager) ensureRootCALocked() (*x509.Certificate, error) {
	cert, _, err := m.ensureRootKeypairLocked()
	return cert, err
}

func (m *Manager) ensureRootKeypairLocked() (*x509.Certificate, *ecdsa.PrivateKey, error) {
	if cert, key, err := m.loadRootKeypairLocked(); err == nil {
		return cert, key, nil
	} else if !errors.Is(err, ErrNoCA) {
		return nil, nil, err
	}
	cert, key, err := generateRootCA()
	if err != nil {
		return nil, nil, err
	}
	if err := os.MkdirAll(m.cfg.Dir, 0o700); err != nil {
		return nil, nil, fmt.Errorf("create certificate directory: %w", err)
	}
	if err := writePEMFile(m.RootCertPath(), "CERTIFICATE", cert.Raw, 0o644); err != nil {
		return nil, nil, err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal root private key: %w", err)
	}
	if err := writePEMFile(m.RootKeyPath(), "EC PRIVATE KEY", keyDER, 0o600); err != nil {
		return nil, nil, err
	}
	m.rootCert = cert
	m.rootKey = key
	m.loadedCA = true
	return cert, key, nil
}

func (m *Manager) loadRootKeypairLocked() (*x509.Certificate, *ecdsa.PrivateKey, error) {
	if m.loadedCA && m.rootCert != nil && m.rootKey != nil {
		return m.rootCert, m.rootKey, nil
	}
	certPEM, err := os.ReadFile(m.RootCertPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, ErrNoCA
		}
		return nil, nil, fmt.Errorf("read root certificate: %w", err)
	}
	keyPEM, err := os.ReadFile(m.RootKeyPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, ErrNoCA
		}
		return nil, nil, fmt.Errorf("read root private key: %w", err)
	}
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil || certBlock.Type != "CERTIFICATE" {
		return nil, nil, fmt.Errorf("invalid root certificate PEM")
	}
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil || keyBlock.Type != "EC PRIVATE KEY" {
		return nil, nil, fmt.Errorf("invalid root private key PEM")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse root certificate: %w", err)
	}
	key, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse root private key: %w", err)
	}
	m.rootCert = cert
	m.rootKey = key
	m.loadedCA = true
	return cert, key, nil
}

func generateRootCA() (*x509.Certificate, *ecdsa.PrivateKey, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate root private key: %w", err)
	}
	serial, err := randomSerial()
	if err != nil {
		return nil, nil, err
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   rootName,
			Organization: []string{"steam-accelerator-core"},
		},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("create root certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, fmt.Errorf("parse generated root certificate: %w", err)
	}
	return cert, key, nil
}

func writePEMFile(path, blockType string, der []byte, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("open %s: %w", filepath.Base(path), err)
	}
	defer file.Close()
	if err := pem.Encode(file, &pem.Block{Type: blockType, Bytes: der}); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
}

func randomSerial() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, fmt.Errorf("generate certificate serial: %w", err)
	}
	return serial, nil
}

func Thumbprint(cert *x509.Certificate) string {
	sum := sha1.Sum(cert.Raw)
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}
