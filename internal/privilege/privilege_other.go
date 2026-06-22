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

func runElevatedHelper(context.Context, HelperRequest) (HelperResponse, error) {
	return HelperResponse{}, fmt.Errorf("%w on this platform", errHelperNotAvailable)
}

func helperStatus() string {
	return fmt.Sprintf("helper pid=%d elevated=false", os.Getpid())
}
