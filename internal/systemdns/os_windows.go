//go:build windows

package systemdns

func newOSPlatform() Platform {
	return windowsPlatform{runner: realPowerShellRunner{}}
}
