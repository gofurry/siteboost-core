//go:build windows

package main

import "golang.org/x/sys/windows/registry"

func discoverPlatformSystemProxies() SystemProxyInfo {
	info := SystemProxyInfo{}
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		info.Warnings = append(info.Warnings, "无法读取 Windows 系统代理设置："+err.Error())
		return info
	}
	defer key.Close()

	if pacURL, _, err := key.GetStringValue("AutoConfigURL"); err == nil && pacURL != "" {
		info.PACURL = pacURL
		info.Warnings = append(info.Warnings, "检测到 PAC，POC 暂不解析")
	}

	enabled, _, err := key.GetIntegerValue("ProxyEnable")
	if err != nil || enabled == 0 {
		return info
	}
	proxyServer, _, err := key.GetStringValue("ProxyServer")
	if err != nil || proxyServer == "" {
		return info
	}
	info.Candidates = parseProxyServer(proxyServer)
	return info
}
