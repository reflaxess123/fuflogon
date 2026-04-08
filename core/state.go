package core

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// State holds everything saved at start time so Stop can correctly undo it.
type State struct {
	OldGateway string   `json:"old_gateway"`
	RealIface  string   `json:"real_iface"`
	OldDNS     []string `json:"old_dns,omitempty"`
	VPSIPs     []string `json:"vps_ips"`
	XrayPID    int      `json:"xray_pid"`
	TunIfIndex int      `json:"tun_if_index,omitempty"`
	RuntimeCfg string   `json:"runtime_cfg"`
}

// SaveState writes the state to .xray-state.json in rootDir.
func SaveState(rootDir string, s State) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(rootDir, StateFileName), data, 0644)
}

// LoadState reads the state from .xray-state.json in rootDir.
func LoadState(rootDir string) (*State, error) {
	data, err := os.ReadFile(filepath.Join(rootDir, StateFileName))
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// DeleteState removes the state file.
func DeleteState(rootDir string) {
	os.Remove(filepath.Join(rootDir, StateFileName))
}
