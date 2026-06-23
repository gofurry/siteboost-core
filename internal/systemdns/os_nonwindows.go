//go:build !windows

package systemdns

import "runtime"

func newOSPlatform() Platform {
	return unsupportedPlatform{name: runtime.GOOS}
}
