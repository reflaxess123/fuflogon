package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
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

// ScanConfigs lists config*.json files in rootDir (excluding dot-files).
func ScanConfigs(rootDir string) []string {
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil
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
		if !strings.HasPrefix(lower, "config") {
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
