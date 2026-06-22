//go:build windows

package hosts

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf16"
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
	if err := os.WriteFile(path, data, mode); err != nil {
		if os.IsPermission(err) {
			if fallbackErr := writeHostsWithPowerShell(path, data); fallbackErr == nil {
				return nil
			} else {
				return fmt.Errorf("%w; PowerShell hosts write fallback failed: %v", err, fallbackErr)
			}
		}
		return err
	}
	return nil
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
		if os.IsPermission(err) {
			if fallbackErr := checkHostsWritableWithPowerShell(path); fallbackErr == nil {
				return nil
			} else {
				return fmt.Errorf("%w; PowerShell hosts writable fallback failed: %v", err, fallbackErr)
			}
		}
		return err
	}
	return file.Close()
}

func checkHostsWritableWithPowerShell(path string) error {
	const script = `
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$path = $env:SITEBOOST_HOSTS_PATH
if ([string]::IsNullOrWhiteSpace($path)) { throw 'SITEBOOST_HOSTS_PATH is empty' }
$stream = [System.IO.File]::Open($path, [System.IO.FileMode]::Open, [System.IO.FileAccess]::ReadWrite, [System.IO.FileShare]::ReadWrite)
try {
}
finally {
    if ($stream -ne $null) { $stream.Close() }
}
`
	return runHostsPowerShell(script, "SITEBOOST_HOSTS_PATH="+path)
}

func writeHostsWithPowerShell(path string, data []byte) error {
	tmp, err := os.CreateTemp("", "siteboost-hosts-*")
	if err != nil {
		return fmt.Errorf("create hosts temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write hosts temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close hosts temp file: %w", err)
	}
	absSource, err := filepath.Abs(tmpPath)
	if err != nil {
		return fmt.Errorf("resolve hosts temp file: %w", err)
	}
	absDest, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve hosts path: %w", err)
	}
	const script = `
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$source = $env:SITEBOOST_HOSTS_SOURCE
$dest = $env:SITEBOOST_HOSTS_DEST
if ([string]::IsNullOrWhiteSpace($source)) { throw 'SITEBOOST_HOSTS_SOURCE is empty' }
if ([string]::IsNullOrWhiteSpace($dest)) { throw 'SITEBOOST_HOSTS_DEST is empty' }
[System.IO.File]::Copy($source, $dest, $true)
`
	return runHostsPowerShell(script,
		"SITEBOOST_HOSTS_SOURCE="+absSource,
		"SITEBOOST_HOSTS_DEST="+absDest,
	)
}

func runHostsPowerShell(script string, extraEnv ...string) error {
	cmd := exec.Command("powershell.exe",
		"-NoLogo",
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-EncodedCommand", hostsPowerShellEncodedCommand(script),
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

func hostsPowerShellEncodedCommand(script string) string {
	encoded := utf16.Encode([]rune(script))
	buf := make([]byte, len(encoded)*2)
	for i, r := range encoded {
		binary.LittleEndian.PutUint16(buf[i*2:], r)
	}
	return base64.StdEncoding.EncodeToString(buf)
}
