package systemproxy

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
)

type MacOSState struct {
	Services []MacOSServiceState `json:"services"`
}

type MacOSServiceState struct {
	Name   string          `json:"name"`
	Web    MacOSProxyState `json:"web"`
	Secure MacOSProxyState `json:"secure"`
	Auto   MacOSAutoState  `json:"auto"`
}

type MacOSProxyState struct {
	Enabled       bool   `json:"enabled"`
	Server        string `json:"server,omitempty"`
	Port          string `json:"port,omitempty"`
	Authenticated bool   `json:"authenticated,omitempty"`
}

type MacOSAutoState struct {
	Enabled bool   `json:"enabled"`
	URL     string `json:"url,omitempty"`
}

type networkSetupRunner interface {
	Run(ctx context.Context, args ...string) (string, error)
}

type execNetworkSetupRunner struct{}

func (execNetworkSetupRunner) Run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "networksetup", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("networksetup %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

type macOSPlatform struct {
	runner networkSetupRunner
}

func (p macOSPlatform) Name() string {
	return "darwin"
}

func (p macOSPlatform) Snapshot(ctx context.Context, cfg Config) (State, error) {
	services, err := p.services(ctx, cfg.Services)
	if err != nil {
		return State{}, err
	}
	state := MacOSState{Services: make([]MacOSServiceState, 0, len(services))}
	for _, service := range services {
		web, err := p.getProxy(ctx, "-getwebproxy", service)
		if err != nil {
			return State{}, err
		}
		secure, err := p.getProxy(ctx, "-getsecurewebproxy", service)
		if err != nil {
			return State{}, err
		}
		if web.Authenticated || secure.Authenticated {
			return State{}, fmt.Errorf("macOS authenticated proxy on service %q cannot be safely restored", service)
		}
		auto, err := p.getAuto(ctx, service)
		if err != nil {
			return State{}, err
		}
		state.Services = append(state.Services, MacOSServiceState{
			Name:   service,
			Web:    web,
			Secure: secure,
			Auto:   auto,
		})
	}
	return State{MacOS: &state}, nil
}

func (p macOSPlatform) ApplyPAC(ctx context.Context, cfg Config) error {
	services, err := p.services(ctx, cfg.Services)
	if err != nil {
		return err
	}
	for _, service := range services {
		if _, err := p.run(ctx, "-setautoproxyurl", service, cfg.PACURL); err != nil {
			return err
		}
		if _, err := p.run(ctx, "-setautoproxystate", service, "on"); err != nil {
			return err
		}
		if _, err := p.run(ctx, "-setwebproxystate", service, "off"); err != nil {
			return err
		}
		if _, err := p.run(ctx, "-setsecurewebproxystate", service, "off"); err != nil {
			return err
		}
	}
	return nil
}

func (p macOSPlatform) ApplySystem(ctx context.Context, cfg Config) error {
	services, err := p.services(ctx, cfg.Services)
	if err != nil {
		return err
	}
	host, port, err := net.SplitHostPort(cfg.ProxyAddr)
	if err != nil {
		return fmt.Errorf("split proxy address: %w", err)
	}
	host = strings.Trim(host, "[]")
	for _, service := range services {
		if _, err := p.run(ctx, "-setwebproxy", service, host, port); err != nil {
			return err
		}
		if _, err := p.run(ctx, "-setsecurewebproxy", service, host, port); err != nil {
			return err
		}
		if _, err := p.run(ctx, "-setwebproxystate", service, "on"); err != nil {
			return err
		}
		if _, err := p.run(ctx, "-setsecurewebproxystate", service, "on"); err != nil {
			return err
		}
		if _, err := p.run(ctx, "-setautoproxystate", service, "off"); err != nil {
			return err
		}
	}
	return nil
}

func (p macOSPlatform) Restore(ctx context.Context, state State) error {
	if state.MacOS == nil {
		return fmt.Errorf("rollback state does not contain macOS proxy settings")
	}
	for _, service := range state.MacOS.Services {
		if service.Web.Server != "" && service.Web.Port != "" {
			if _, err := p.run(ctx, "-setwebproxy", service.Name, service.Web.Server, service.Web.Port); err != nil {
				return err
			}
		}
		if _, err := p.run(ctx, "-setwebproxystate", service.Name, onOff(service.Web.Enabled)); err != nil {
			return err
		}
		if service.Secure.Server != "" && service.Secure.Port != "" {
			if _, err := p.run(ctx, "-setsecurewebproxy", service.Name, service.Secure.Server, service.Secure.Port); err != nil {
				return err
			}
		}
		if _, err := p.run(ctx, "-setsecurewebproxystate", service.Name, onOff(service.Secure.Enabled)); err != nil {
			return err
		}
		if service.Auto.URL != "" {
			if _, err := p.run(ctx, "-setautoproxyurl", service.Name, service.Auto.URL); err != nil {
				return err
			}
		}
		if _, err := p.run(ctx, "-setautoproxystate", service.Name, onOff(service.Auto.Enabled)); err != nil {
			return err
		}
	}
	return nil
}

func (p macOSPlatform) services(ctx context.Context, configured []string) ([]string, error) {
	if len(configured) > 0 {
		services := make([]string, 0, len(configured))
		for _, service := range configured {
			service = strings.TrimSpace(service)
			if service != "" {
				services = append(services, service)
			}
		}
		if len(services) == 0 {
			return nil, fmt.Errorf("no macOS network services configured")
		}
		return services, nil
	}
	out, err := p.run(ctx, "-listallnetworkservices")
	if err != nil {
		return nil, err
	}
	var services []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "asterisk") {
			continue
		}
		if strings.HasPrefix(line, "*") {
			continue
		}
		services = append(services, line)
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("no enabled macOS network services found")
	}
	return services, nil
}

func (p macOSPlatform) getProxy(ctx context.Context, command, service string) (MacOSProxyState, error) {
	out, err := p.run(ctx, command, service)
	if err != nil {
		return MacOSProxyState{}, err
	}
	values := parseNetworkSetupOutput(out)
	return MacOSProxyState{
		Enabled:       parseBool(values["enabled"]),
		Server:        values["server"],
		Port:          values["port"],
		Authenticated: parseBool(values["authenticated proxy enabled"]),
	}, nil
}

func (p macOSPlatform) getAuto(ctx context.Context, service string) (MacOSAutoState, error) {
	out, err := p.run(ctx, "-getautoproxyurl", service)
	if err != nil {
		return MacOSAutoState{}, err
	}
	values := parseNetworkSetupOutput(out)
	return MacOSAutoState{
		Enabled: parseBool(values["enabled"]),
		URL:     values["url"],
	}, nil
}

func (p macOSPlatform) run(ctx context.Context, args ...string) (string, error) {
	if p.runner == nil {
		return "", fmt.Errorf("networksetup runner is required")
	}
	return p.runner.Run(ctx, args...)
}

func parseNetworkSetupOutput(out string) map[string]string {
	values := make(map[string]string)
	for _, line := range strings.Split(out, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		values[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
	}
	return values
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "yes", "on", "1", "true", "enabled":
		return true
	default:
		return false
	}
}

func onOff(value bool) string {
	if value {
		return "on"
	}
	return "off"
}
