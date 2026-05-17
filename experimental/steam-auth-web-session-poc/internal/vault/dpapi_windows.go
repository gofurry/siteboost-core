//go:build windows

package vault

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

type dpapiVault struct {
	dir string
}

func New() (Vault, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return &dpapiVault{dir: filepath.Join(base, "SteamScope", "steam-auth-web-session-poc", "vault")}, nil
}

func (v *dpapiVault) Put(key string, value string) error {
	protected, err := protect([]byte(value))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(v.dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(v.path(key), []byte(base64.StdEncoding.EncodeToString(protected)), 0o600)
}

func (v *dpapiVault) Get(key string) (string, error) {
	content, err := os.ReadFile(v.path(key))
	if err != nil {
		return "", err
	}
	protected, err := base64.StdEncoding.DecodeString(string(content))
	if err != nil {
		return "", err
	}
	plain, err := unprotect(protected)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (v *dpapiVault) Delete(key string) error {
	err := os.Remove(v.path(key))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (v *dpapiVault) path(key string) string {
	return filepath.Join(v.dir, key+".dpapi")
}

func protect(data []byte) ([]byte, error) {
	in := bytesToBlob(data)
	var out windows.DataBlob
	if err := windows.CryptProtectData(in, nil, nil, 0, nil, 0, &out); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return blobToBytes(&out), nil
}

func unprotect(data []byte) ([]byte, error) {
	in := bytesToBlob(data)
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(in, nil, nil, 0, nil, 0, &out); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return blobToBytes(&out), nil
}

func bytesToBlob(data []byte) *windows.DataBlob {
	if len(data) == 0 {
		return &windows.DataBlob{}
	}
	return &windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
}

func blobToBytes(blob *windows.DataBlob) []byte {
	if blob == nil || blob.Data == nil || blob.Size == 0 {
		return nil
	}
	return append([]byte(nil), unsafe.Slice(blob.Data, blob.Size)...)
}
