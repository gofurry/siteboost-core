//go:build windows

package certstore

import (
	"testing"

	"github.com/gofurry/go-steam-core/internal/config"
	"golang.org/x/sys/windows"
)

func TestWindowsRootStoreOpenFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		scope     string
		writable  bool
		wantSet   uint32
		wantClear uint32
	}{
		{
			name:      "machine read only",
			scope:     config.CertStoreMachine,
			writable:  false,
			wantSet:   windows.CERT_SYSTEM_STORE_LOCAL_MACHINE | windows.CERT_STORE_OPEN_EXISTING_FLAG | windows.CERT_STORE_READONLY_FLAG,
			wantClear: windows.CERT_STORE_MAXIMUM_ALLOWED_FLAG,
		},
		{
			name:      "machine writable",
			scope:     config.CertStoreMachine,
			writable:  true,
			wantSet:   windows.CERT_SYSTEM_STORE_LOCAL_MACHINE,
			wantClear: windows.CERT_STORE_READONLY_FLAG | windows.CERT_STORE_OPEN_EXISTING_FLAG | windows.CERT_STORE_MAXIMUM_ALLOWED_FLAG,
		},
		{
			name:      "user writable",
			scope:     config.CertStoreUser,
			writable:  true,
			wantSet:   windows.CERT_SYSTEM_STORE_CURRENT_USER,
			wantClear: windows.CERT_STORE_READONLY_FLAG | windows.CERT_STORE_OPEN_EXISTING_FLAG | windows.CERT_STORE_MAXIMUM_ALLOWED_FLAG,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := windowsRootStoreOpenFlags(tt.scope, tt.writable)
			if err != nil {
				t.Fatalf("windowsRootStoreOpenFlags() error = %v", err)
			}
			if got&tt.wantSet != tt.wantSet {
				t.Fatalf("windowsRootStoreOpenFlags() = %#x, missing %#x", got, tt.wantSet&^got)
			}
			if got&tt.wantClear != 0 {
				t.Fatalf("windowsRootStoreOpenFlags() = %#x, unexpectedly set %#x", got, got&tt.wantClear)
			}
		})
	}
}

func TestPowerShellEncodedCommandUsesUTF16LE(t *testing.T) {
	t.Parallel()

	if got, want := powerShellEncodedCommand("abc"), "YQBiAGMA"; got != want {
		t.Fatalf("powerShellEncodedCommand() = %q, want %q", got, want)
	}
}
