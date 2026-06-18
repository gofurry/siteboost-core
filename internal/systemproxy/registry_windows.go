//go:build windows

package systemproxy

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const internetSettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

type realWindowsBackend struct{}

func (realWindowsBackend) Snapshot() (WindowsState, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.QUERY_VALUE)
	if err != nil {
		return WindowsState{}, fmt.Errorf("open internet settings: %w", err)
	}
	defer key.Close()

	proxyEnable, err := getDWORDValue(key, "ProxyEnable")
	if err != nil {
		return WindowsState{}, err
	}
	proxyServer, err := getStringValue(key, "ProxyServer")
	if err != nil {
		return WindowsState{}, err
	}
	autoConfigURL, err := getStringValue(key, "AutoConfigURL")
	if err != nil {
		return WindowsState{}, err
	}
	return WindowsState{
		ProxyEnable:   proxyEnable,
		ProxyServer:   proxyServer,
		AutoConfigURL: autoConfigURL,
	}, nil
}

func (realWindowsBackend) ApplyPAC(pacURL string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open internet settings: %w", err)
	}
	defer key.Close()

	if err := key.SetStringValue("AutoConfigURL", pacURL); err != nil {
		return fmt.Errorf("set AutoConfigURL: %w", err)
	}
	if err := key.SetDWordValue("ProxyEnable", 0); err != nil {
		return fmt.Errorf("set ProxyEnable: %w", err)
	}
	return notifyWindowsProxyChanged()
}

func (realWindowsBackend) ApplySystem(proxyAddr string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open internet settings: %w", err)
	}
	defer key.Close()

	if err := key.SetStringValue("ProxyServer", "http="+proxyAddr+";https="+proxyAddr); err != nil {
		return fmt.Errorf("set ProxyServer: %w", err)
	}
	if err := key.SetDWordValue("ProxyEnable", 1); err != nil {
		return fmt.Errorf("set ProxyEnable: %w", err)
	}
	if err := deleteValue(key, "AutoConfigURL"); err != nil {
		return err
	}
	return notifyWindowsProxyChanged()
}

func (realWindowsBackend) Restore(state WindowsState) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open internet settings: %w", err)
	}
	defer key.Close()

	if err := restoreDWORDValue(key, "ProxyEnable", state.ProxyEnable); err != nil {
		return err
	}
	if err := restoreStringValue(key, "ProxyServer", state.ProxyServer); err != nil {
		return err
	}
	if err := restoreStringValue(key, "AutoConfigURL", state.AutoConfigURL); err != nil {
		return err
	}
	return notifyWindowsProxyChanged()
}

func getDWORDValue(key registry.Key, name string) (WindowsDWORD, error) {
	value, _, err := key.GetIntegerValue(name)
	if errors.Is(err, registry.ErrNotExist) {
		return WindowsDWORD{}, nil
	}
	if err != nil {
		return WindowsDWORD{}, fmt.Errorf("read %s: %w", name, err)
	}
	return WindowsDWORD{Exists: true, Value: value}, nil
}

func getStringValue(key registry.Key, name string) (WindowsString, error) {
	value, _, err := key.GetStringValue(name)
	if errors.Is(err, registry.ErrNotExist) {
		return WindowsString{}, nil
	}
	if err != nil {
		return WindowsString{}, fmt.Errorf("read %s: %w", name, err)
	}
	return WindowsString{Exists: true, Value: value}, nil
}

func restoreDWORDValue(key registry.Key, name string, value WindowsDWORD) error {
	if !value.Exists {
		return deleteValue(key, name)
	}
	if err := key.SetDWordValue(name, uint32(value.Value)); err != nil {
		return fmt.Errorf("restore %s: %w", name, err)
	}
	return nil
}

func restoreStringValue(key registry.Key, name string, value WindowsString) error {
	if !value.Exists {
		return deleteValue(key, name)
	}
	if err := key.SetStringValue(name, value.Value); err != nil {
		return fmt.Errorf("restore %s: %w", name, err)
	}
	return nil
}

func deleteValue(key registry.Key, name string) error {
	err := key.DeleteValue(name)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete %s: %w", name, err)
	}
	return nil
}

func notifyWindowsProxyChanged() error {
	wininet := windows.NewLazySystemDLL("wininet.dll")
	internetSetOption := wininet.NewProc("InternetSetOptionW")
	for _, option := range []uintptr{39, 37} {
		ret, _, callErr := internetSetOption.Call(0, option, 0, 0)
		if ret == 0 {
			return fmt.Errorf("notify WinINet option %d: %w", option, callErr)
		}
	}

	user32 := windows.NewLazySystemDLL("user32.dll")
	sendMessageTimeout := user32.NewProc("SendMessageTimeoutW")
	setting, err := windows.UTF16PtrFromString(internetSettingsKey)
	if err != nil {
		return err
	}
	var result uintptr
	ret, _, callErr := sendMessageTimeout.Call(
		0xffff,
		0x001a,
		0,
		uintptr(unsafe.Pointer(setting)),
		0x0002,
		5000,
		uintptr(unsafe.Pointer(&result)),
	)
	if ret == 0 {
		return fmt.Errorf("broadcast internet settings change: %w", callErr)
	}
	return nil
}
