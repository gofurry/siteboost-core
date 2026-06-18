package systemproxy

import (
	"context"
	"fmt"
)

type WindowsState struct {
	ProxyEnable   WindowsDWORD  `json:"proxy_enable"`
	ProxyServer   WindowsString `json:"proxy_server"`
	AutoConfigURL WindowsString `json:"auto_config_url"`
}

type WindowsDWORD struct {
	Exists bool   `json:"exists"`
	Value  uint64 `json:"value"`
}

type WindowsString struct {
	Exists bool   `json:"exists"`
	Value  string `json:"value"`
}

type windowsBackend interface {
	Snapshot() (WindowsState, error)
	ApplyPAC(pacURL string) error
	ApplySystem(proxyAddr string) error
	Restore(state WindowsState) error
}

type windowsPlatform struct {
	backend windowsBackend
}

func (p windowsPlatform) Name() string {
	return "windows"
}

func (p windowsPlatform) Snapshot(context.Context, Config) (State, error) {
	if p.backend == nil {
		return State{}, fmt.Errorf("windows backend is required")
	}
	state, err := p.backend.Snapshot()
	if err != nil {
		return State{}, err
	}
	return State{Windows: &state}, nil
}

func (p windowsPlatform) ApplyPAC(_ context.Context, cfg Config) error {
	if p.backend == nil {
		return fmt.Errorf("windows backend is required")
	}
	return p.backend.ApplyPAC(cfg.PACURL)
}

func (p windowsPlatform) ApplySystem(_ context.Context, cfg Config) error {
	if p.backend == nil {
		return fmt.Errorf("windows backend is required")
	}
	return p.backend.ApplySystem(cfg.ProxyAddr)
}

func (p windowsPlatform) Restore(_ context.Context, state State) error {
	if p.backend == nil {
		return fmt.Errorf("windows backend is required")
	}
	if state.Windows == nil {
		return fmt.Errorf("rollback state does not contain windows proxy settings")
	}
	return p.backend.Restore(*state.Windows)
}
