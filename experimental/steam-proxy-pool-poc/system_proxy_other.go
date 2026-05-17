//go:build !windows

package main

func discoverPlatformSystemProxies() SystemProxyInfo {
	return SystemProxyInfo{}
}
