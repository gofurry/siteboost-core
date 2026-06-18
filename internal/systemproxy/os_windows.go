//go:build windows

package systemproxy

func newOSPlatform() Platform {
	return windowsPlatform{backend: realWindowsBackend{}}
}
