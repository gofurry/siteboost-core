//go:build !windows

package privilege

import (
	"context"
	"fmt"
	"os"
)

func IsElevated() bool {
	return false
}

func HasSystemPrivileges() bool {
	return false
}

func RelaunchElevated(args []string) error {
	return fmt.Errorf("%w on this platform", errHelperNotAvailable)
}

func ensureAppHostStarted(context.Context) error {
	return fmt.Errorf("%w on this platform", errHelperNotAvailable)
}

func RunAppHostService([]string, io.Writer, io.Writer) error {
	return fmt.Errorf("%w on this platform", errHelperNotAvailable)
}

func InstallAppHostService(context.Context) error {
	return fmt.Errorf("%w on this platform", errHelperNotAvailable)
}

func UninstallAppHostService(context.Context) error {
	return fmt.Errorf("%w on this platform", errHelperNotAvailable)
}

func StartAppHostService(context.Context) error {
	return fmt.Errorf("%w on this platform", errHelperNotAvailable)
}

func StopAppHostService(context.Context) error {
	return fmt.Errorf("%w on this platform", errHelperNotAvailable)
}

func AppHostServiceStatus(context.Context) (string, error) {
	return "", fmt.Errorf("%w on this platform", errHelperNotAvailable)
}

func runElevatedHelper(context.Context, HelperRequest) (HelperResponse, error) {
	return HelperResponse{}, fmt.Errorf("%w on this platform", errHelperNotAvailable)
}

func helperStatus() string {
	return fmt.Sprintf("helper pid=%d elevated=false", os.Getpid())
}
