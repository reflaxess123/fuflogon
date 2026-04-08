package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/reflaxess123/fuflogon/core"
)

// IsProcessRunning returns true if a process with the given PID is alive.
func IsProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	return isProcessAlive(pid)
}

// KillProcess kills the process with the given PID.
func KillProcess(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}

// StopExistingXray kills any running xray process found via .xray.pid.
func StopExistingXray(rootDir string) {
	pidPath := filepath.Join(rootDir, core.PidFileName)
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return
	}
	var pid int
	if n, _ := fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &pid); n == 1 && pid > 0 {
		KillProcess(pid)
	}
}
