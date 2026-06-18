//go:build !windows

package systemproxy

import "runtime"

func newOSPlatform() Platform {
	if runtime.GOOS == "darwin" {
		return macOSPlatform{runner: execNetworkSetupRunner{}}
	}
	return unsupportedPlatform{name: runtime.GOOS}
}
