//go:build windows

package hosts

import (
	"os"
	"runtime"
)

type osPlatform struct{}

func newOSPlatform() Platform {
	return osPlatform{}
}

func (osPlatform) Name() string {
	return runtime.GOOS
}

func (osPlatform) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (osPlatform) WriteFile(path string, data []byte, mode os.FileMode) error {
	return os.WriteFile(path, data, mode)
}

func (osPlatform) FileMode(path string) (os.FileMode, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Mode().Perm(), nil
}

func (osPlatform) CheckWritable(path string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	return file.Close()
}
