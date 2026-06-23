package systemdns

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type powerShellRunner interface {
	RunPowerShell(ctx context.Context, script string) (string, error)
}

type windowsPlatform struct {
	runner powerShellRunner
}

type windowsDNSInterface struct {
	InterfaceAlias   string   `json:"InterfaceAlias"`
	InterfaceIndex   int      `json:"InterfaceIndex"`
	InterfaceGUID    string   `json:"InterfaceGuid"`
	AddressFamily    string   `json:"AddressFamily"`
	StaticNameServer string   `json:"StaticNameServer"`
	Servers          []string `json:"Servers"`
}

func (p windowsPlatform) Name() string {
	return "windows"
}

func (p windowsPlatform) Snapshot(ctx context.Context, cfg Config) (State, error) {
	if p.runner == nil {
		return State{}, fmt.Errorf("windows PowerShell runner is required")
	}
	out, err := p.runner.RunPowerShell(ctx, windowsSnapshotScript)
	if err != nil {
		return State{}, err
	}
	items, err := decodeWindowsDNSInterfaces(out)
	if err != nil {
		return State{}, err
	}
	interfaces, err := selectWindowsDNSInterfaces(items, cfg.Interfaces)
	if err != nil {
		return State{}, err
	}
	return State{Interfaces: interfaces}, nil
}

func (p windowsPlatform) Apply(ctx context.Context, cfg Config, state State) error {
	if p.runner == nil {
		return fmt.Errorf("windows PowerShell runner is required")
	}
	indexes := interfaceIndexes(state.Interfaces)
	if len(indexes) == 0 {
		return fmt.Errorf("system DNS rollback state has no interfaces")
	}
	_, err := p.runner.RunPowerShell(ctx, buildWindowsApplyScript(indexes, cfg.ServerIPs))
	return err
}

func (p windowsPlatform) Restore(ctx context.Context, state State) error {
	if p.runner == nil {
		return fmt.Errorf("windows PowerShell runner is required")
	}
	if len(state.Interfaces) == 0 {
		return fmt.Errorf("system DNS rollback state has no interfaces")
	}
	_, err := p.runner.RunPowerShell(ctx, buildWindowsRestoreScript(state.Interfaces))
	return err
}

func decodeWindowsDNSInterfaces(out string) ([]windowsDNSInterface, error) {
	out = strings.TrimSpace(out)
	if out == "" {
		return nil, nil
	}
	var items []windowsDNSInterface
	if err := json.Unmarshal([]byte(out), &items); err == nil {
		return items, nil
	}
	var item windowsDNSInterface
	if err := json.Unmarshal([]byte(out), &item); err != nil {
		return nil, fmt.Errorf("parse Windows DNS interface snapshot: %w", err)
	}
	return []windowsDNSInterface{item}, nil
}

func selectWindowsDNSInterfaces(items []windowsDNSInterface, selectors []string) ([]InterfaceState, error) {
	if len(selectors) == 0 {
		return nil, fmt.Errorf("system DNS interfaces are required")
	}
	selected := make([]InterfaceState, 0, len(selectors))
	seenIndexes := make(map[int]struct{}, len(selectors))
	for _, selector := range selectors {
		matched := false
		for _, item := range items {
			if !windowsInterfaceMatches(item, selector) {
				continue
			}
			matched = true
			if _, ok := seenIndexes[item.InterfaceIndex]; ok {
				continue
			}
			seenIndexes[item.InterfaceIndex] = struct{}{}
			selected = append(selected, interfaceStateFromWindows(item))
		}
		if !matched {
			return nil, fmt.Errorf("system DNS interface %q was not found", selector)
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no system DNS interfaces selected")
	}
	return selected, nil
}

func windowsInterfaceMatches(item windowsDNSInterface, selector string) bool {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return false
	}
	if strconv.Itoa(item.InterfaceIndex) == selector {
		return true
	}
	if strings.EqualFold(item.InterfaceAlias, selector) {
		return true
	}
	return strings.EqualFold(normalizeWindowsGUID(item.InterfaceGUID), normalizeWindowsGUID(selector))
}

func interfaceStateFromWindows(item windowsDNSInterface) InterfaceState {
	staticServers := parseWindowsServerList(item.StaticNameServer)
	source := "dhcp"
	if len(staticServers) > 0 {
		source = "static"
	}
	return InterfaceState{
		InterfaceAlias:   strings.TrimSpace(item.InterfaceAlias),
		InterfaceIndex:   item.InterfaceIndex,
		InterfaceGUID:    normalizeWindowsGUID(item.InterfaceGUID),
		AddressFamily:    strings.TrimSpace(item.AddressFamily),
		Source:           source,
		Servers:          cleanStrings(item.Servers),
		StaticServers:    staticServers,
		StaticNameServer: strings.TrimSpace(item.StaticNameServer),
	}
}

func parseWindowsServerList(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\r' || r == '\n'
	})
	return cleanStrings(fields)
}

func normalizeWindowsGUID(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "{")
	value = strings.TrimSuffix(value, "}")
	return strings.ToLower(value)
}

func interfaceIndexes(interfaces []InterfaceState) []int {
	indexes := make([]int, 0, len(interfaces))
	for _, iface := range interfaces {
		if iface.InterfaceIndex > 0 {
			indexes = append(indexes, iface.InterfaceIndex)
		}
	}
	return indexes
}

func buildWindowsApplyScript(indexes []int, serverIPs []string) string {
	return "$ErrorActionPreference = 'Stop'\n" +
		"$servers = " + psStringArray(serverIPs) + "\n" +
		"$indexes = " + psIntArray(indexes) + "\n" +
		"foreach ($idx in $indexes) {\n" +
		"  Set-DnsClientServerAddress -InterfaceIndex ([int]$idx) -ServerAddresses $servers\n" +
		"}\n"
}

func buildWindowsRestoreScript(interfaces []InterfaceState) string {
	items := make([]string, 0, len(interfaces))
	for _, iface := range interfaces {
		servers := iface.StaticServers
		if len(servers) == 0 {
			servers = parseWindowsServerList(iface.StaticNameServer)
		}
		source := strings.ToLower(strings.TrimSpace(iface.Source))
		if source != "static" {
			source = "dhcp"
		}
		items = append(items, fmt.Sprintf("@{ InterfaceIndex = %d; Source = %s; Servers = %s }",
			iface.InterfaceIndex,
			psQuote(source),
			psStringArray(servers),
		))
	}
	return "$ErrorActionPreference = 'Stop'\n" +
		"$items = @(\n  " + strings.Join(items, ",\n  ") + "\n)\n" +
		"foreach ($item in $items) {\n" +
		"  if ($item.Source -eq 'dhcp') {\n" +
		"    Set-DnsClientServerAddress -InterfaceIndex ([int]$item.InterfaceIndex) -ResetServerAddresses\n" +
		"  } else {\n" +
		"    Set-DnsClientServerAddress -InterfaceIndex ([int]$item.InterfaceIndex) -ServerAddresses $item.Servers\n" +
		"  }\n" +
		"}\n"
}

func psStringArray(values []string) string {
	if len(values) == 0 {
		return "@()"
	}
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, psQuote(value))
	}
	return "@(" + strings.Join(quoted, ", ") + ")"
}

func psIntArray(values []int) string {
	if len(values) == 0 {
		return "@()"
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(value))
	}
	return "@(" + strings.Join(parts, ", ") + ")"
}

func psQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

const windowsSnapshotScript = `
$ErrorActionPreference = 'Stop'
$adapterMap = @{}
try {
  Get-NetAdapter | ForEach-Object {
    $adapterMap[[int]$_.ifIndex] = [PSCustomObject]@{
      Name = [string]$_.Name
      Guid = [string]$_.InterfaceGuid
    }
  }
} catch {}
$items = @(Get-DnsClientServerAddress -AddressFamily IPv4 | ForEach-Object {
  $idx = [int]$_.InterfaceIndex
  $alias = [string]$_.InterfaceAlias
  $guid = ''
  if ($adapterMap.ContainsKey($idx)) {
    if ([string]::IsNullOrWhiteSpace($alias)) {
      $alias = [string]$adapterMap[$idx].Name
    }
    $guid = [string]$adapterMap[$idx].Guid
  }
  $staticNameServer = ''
  if (-not [string]::IsNullOrWhiteSpace($guid)) {
    $path = 'HKLM:\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces\{' + $guid + '}'
    try {
      $staticNameServer = [string](Get-ItemProperty -LiteralPath $path -Name NameServer -ErrorAction Stop).NameServer
    } catch {}
  }
  [PSCustomObject]@{
    InterfaceAlias = $alias
    InterfaceIndex = $idx
    InterfaceGuid = $guid
    AddressFamily = 'IPv4'
    StaticNameServer = $staticNameServer
    Servers = @($_.ServerAddresses)
  }
})
ConvertTo-Json -InputObject $items -Depth 6 -Compress
`
