//go:build windows

package hosts

import "testing"

func TestHostsPowerShellEncodedCommandUsesUTF16LE(t *testing.T) {
	t.Parallel()

	if got, want := hostsPowerShellEncodedCommand("abc"), "YQBiAGMA"; got != want {
		t.Fatalf("hostsPowerShellEncodedCommand() = %q, want %q", got, want)
	}
}
