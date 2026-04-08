//go:build windows

package main

import (
	_ "embed"
	"os"
	"path/filepath"

	"github.com/reflaxess123/fuflogon/core"
)

//go:embed assets/wintun.dll
var wintunDLLBytes []byte

// extractWintun writes the embedded wintun.dll to rootDir if it's not already
// present. Called once at startup so the user never has to bundle it manually.
func extractWintun(rootDir string) {
	dst := filepath.Join(rootDir, "wintun.dll")
	if core.FileExists(dst) {
		return
	}
	if err := os.WriteFile(dst, wintunDLLBytes, 0644); err != nil {
		core.Logf("[BOOTSTRAP] failed to extract wintun.dll: %v", err)
		return
	}
	core.Logf("[BOOTSTRAP] extracted wintun.dll (%d bytes)", len(wintunDLLBytes))
}
