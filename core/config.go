package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// GetRootDir returns the directory where the executable lives.
func GetRootDir() string {
	exe, err := os.Executable()
	if err != nil {
		cwd, _ := os.Getwd()
		return cwd
	}
	return filepath.Dir(exe)
}

// FindXrayBinary searches for the xray binary for the current platform in rootDir.
func FindXrayBinary(rootDir string) (string, error) {
	for _, name := range XrayCandidates() {
		p := filepath.Join(rootDir, name)
		if FileExists(p) {
			return p, nil
		}
	}
	return "", fmt.Errorf("xray binary not found in %s (looked for %v)",
		rootDir, XrayCandidates())
}

// XrayCandidates returns candidate binary names for the current platform.
func XrayCandidates() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"xray.exe"}
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return []string{"xray-linux-amd64", "xray"}
		case "arm64":
			return []string{"xray-linux-arm64", "xray"}
		}
		return []string{"xray"}
	case "darwin":
		switch runtime.GOARCH {
		case "arm64":
			return []string{"xray-darwin-arm64", "xray"}
		case "amd64":
			return []string{"xray-darwin-amd64", "xray"}
		}
		return []string{"xray"}
	}
	return []string{"xray"}
}

// ScanConfigs lists every *.json file in rootDir that looks like an xray
// config (skips dot-files, package.json, tsconfig*.json, wails.json — anything
// that obviously isn't a tunnel config).
func ScanConfigs(rootDir string) []string {
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil
	}

	// Names that occasionally show up next to fuflogon.exe but are NOT xray
	// configs. Lowercased for comparison.
	skip := map[string]bool{
		"package.json":      true,
		"package-lock.json": true,
		"tsconfig.json":     true,
		"wails.json":        true,
	}

	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".json") {
			continue
		}
		if strings.HasPrefix(name, ".") {
			continue
		}
		if skip[lower] {
			continue
		}
		out = append(out, filepath.Join(rootDir, name))
	}
	sort.Strings(out)
	return out
}

// PickDefaultConfig prefers DefaultCfgName from the list.
func PickDefaultConfig(list []string) string {
	for _, p := range list {
		if filepath.Base(p) == DefaultCfgName {
			return p
		}
	}
	return list[0]
}

// OutboundInfo describes one outbound block from the xray config.
type OutboundInfo struct {
	Tag      string `json:"tag"`
	Protocol string `json:"protocol"`
	Address  string `json:"address,omitempty"`
	Port     int    `json:"port,omitempty"`
	Network  string `json:"network,omitempty"`
	Security string `json:"security,omitempty"`
}

// RoutingRule is one entry from routing.rules in the config. All fields are
// kept as strings/[]string for the UI; the parser tolerates port as either
// number or string in the source JSON.
type RoutingRule struct {
	OutboundTag string   `json:"outboundTag"`
	Type        string   `json:"type"`
	Domain      []string `json:"domain,omitempty"`
	IP          []string `json:"ip,omitempty"`
	Port        string   `json:"port,omitempty"`
	Network     string   `json:"network,omitempty"`
	Source      []string `json:"source,omitempty"`
	Protocol    []string `json:"protocol,omitempty"`
	InboundTag  []string `json:"inboundTag,omitempty"`
}

// ConfigInfo is the parsed view of an xray config — used by the UI.
type ConfigInfo struct {
	Outbounds []OutboundInfo `json:"outbounds"`
	Rules     []RoutingRule  `json:"rules"`
	// Default is the outbound xray will use if no routing rule matches.
	// Per xray docs this is the FIRST outbound in the array.
	Default string `json:"default"`
	// Primary is the most "interesting" proxy outbound — used by the UI to
	// highlight the active VPN tunnel. Falls back to Default if none found.
	Primary string `json:"primary"`
}

// asString converts an arbitrary JSON value to a string. Numbers become their
// decimal representation, strings stay as is, anything else becomes "".
func asString(v interface{}) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	}
	return ""
}

// asStringSlice accepts a JSON array of strings, a single string, or anything
// else. Returns nil if no usable strings are found.
func asStringSlice(v interface{}) []string {
	switch x := v.(type) {
	case []interface{}:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if s := asString(item); s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if x == "" {
			return nil
		}
		return []string{x}
	}
	return nil
}

// ParseConfigInfo reads and parses a config file into a UI-friendly view.
// The parser is intentionally lenient: it tolerates fields with unexpected
// types (e.g. port as number vs string) by going through map[string]interface{}.
func ParseConfigInfo(cfgPath string) (*ConfigInfo, error) {
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, err
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	info := &ConfigInfo{
		Outbounds: []OutboundInfo{},
		Rules:     []RoutingRule{},
	}

	// --- outbounds ---
	if obs, ok := raw["outbounds"].([]interface{}); ok {
		for _, item := range obs {
			ob, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			info.Outbounds = append(info.Outbounds, parseOutbound(ob))
		}
	}

	// --- routing.rules ---
	if routing, ok := raw["routing"].(map[string]interface{}); ok {
		if rules, ok := routing["rules"].([]interface{}); ok {
			for _, item := range rules {
				r, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				info.Rules = append(info.Rules, parseRule(r))
			}
		}
	}

	// Default outbound: per xray docs, the first outbound in the array is the
	// fallback for traffic that no rule matches.
	if len(info.Outbounds) > 0 {
		info.Default = info.Outbounds[0].Tag
	}

	// Primary outbound: first non-direct/non-block proxy outbound, used by the
	// UI to highlight the active VPN tunnel.
	for _, o := range info.Outbounds {
		if o.Tag != "" && o.Tag != "direct" && o.Tag != "block" &&
			o.Protocol != "freedom" && o.Protocol != "blackhole" && o.Protocol != "dns" {
			info.Primary = o.Tag
			break
		}
	}
	if info.Primary == "" {
		info.Primary = info.Default
	}

	Logf("[INFO] parsed config: %d outbounds, %d rules, default=%q primary=%q",
		len(info.Outbounds), len(info.Rules), info.Default, info.Primary)

	return info, nil
}

func parseOutbound(o map[string]interface{}) OutboundInfo {
	ob := OutboundInfo{
		Tag:      asString(o["tag"]),
		Protocol: asString(o["protocol"]),
	}
	if ss, ok := o["streamSettings"].(map[string]interface{}); ok {
		ob.Network = asString(ss["network"])
		ob.Security = asString(ss["security"])
	}
	if settings, ok := o["settings"].(map[string]interface{}); ok {
		// vnext (vless/vmess) or servers (shadowsocks/trojan)
		extract := func(arr interface{}) {
			items, ok := arr.([]interface{})
			if !ok || len(items) == 0 {
				return
			}
			first, ok := items[0].(map[string]interface{})
			if !ok {
				return
			}
			ob.Address = asString(first["address"])
			if p, ok := first["port"].(float64); ok {
				ob.Port = int(p)
			}
		}
		extract(settings["vnext"])
		if ob.Address == "" {
			extract(settings["servers"])
		}
	}
	return ob
}

func parseRule(r map[string]interface{}) RoutingRule {
	return RoutingRule{
		OutboundTag: asString(r["outboundTag"]),
		Type:        asString(r["type"]),
		Domain:      asStringSlice(r["domain"]),
		IP:          asStringSlice(r["ip"]),
		Port:        asString(r["port"]),
		Network:     asString(r["network"]),
		Source:      asStringSlice(r["source"]),
		Protocol:    asStringSlice(r["protocol"]),
		InboundTag:  asStringSlice(r["inboundTag"]),
	}
}

// ExtractServerIPs parses a config file and returns all outbound server IPv4 addresses.
func ExtractServerIPs(cfgPath string) ([]string, error) {
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, err
	}
	var cfg struct {
		Outbounds []struct {
			Protocol string `json:"protocol"`
			Settings struct {
				VNext []struct {
					Address string `json:"address"`
				} `json:"vnext"`
				Servers []struct {
					Address string `json:"address"`
				} `json:"servers"`
			} `json:"settings"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	ipRe := regexp.MustCompile(`^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$`)
	seen := make(map[string]bool)
	var ips []string
	for _, o := range cfg.Outbounds {
		var addrs []string
		for _, v := range o.Settings.VNext {
			addrs = append(addrs, v.Address)
		}
		for _, s := range o.Settings.Servers {
			addrs = append(addrs, s.Address)
		}
		for _, a := range addrs {
			if a != "" && ipRe.MatchString(a) && !seen[a] {
				seen[a] = true
				ips = append(ips, a)
			}
		}
	}
	return ips, nil
}

// WriteRuntimeConfig copies srcPath to dstPath replacing all "interface" values with realIface.
func WriteRuntimeConfig(srcPath, dstPath, realIface string) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	re := regexp.MustCompile(`"interface":\s*"[^"]*"`)
	replaced := re.ReplaceAll(data, []byte(fmt.Sprintf(`"interface": "%s"`, realIface)))
	return os.WriteFile(dstPath, replaced, 0644)
}

// FileExists reports whether a file or directory exists at p.
func FileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// Truncate returns s truncated to n characters with "..." suffix if needed.
func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
