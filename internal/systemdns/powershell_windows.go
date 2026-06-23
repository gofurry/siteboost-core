//go:build windows

package systemdns

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

type realPowerShellRunner struct{}

func (realPowerShellRunner) RunPowerShell(ctx context.Context, script string) (string, error) {
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return stdout.String(), fmt.Errorf("%w: %s", err, stderr.String())
		}
		return stdout.String(), err
	}
	return stdout.String(), nil
}
