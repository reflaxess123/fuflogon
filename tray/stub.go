//go:build !windows

package tray

import (
	"fmt"
	"os"
)

// ShowError prints to stderr on non-Windows platforms.
func ShowError(msg string) { fmt.Fprintln(os.Stderr, "ERROR: "+msg) }

// ShowInfo prints to stderr on non-Windows platforms.
func ShowInfo(msg string) { fmt.Fprintln(os.Stderr, msg) }

func showError(msg string) { ShowError(msg) }
func showInfo(msg string)  { ShowInfo(msg) }

// RunTray is not supported on non-Windows platforms.
func RunTray() {
	fmt.Fprintln(os.Stderr, "tray mode is windows-only; use CLI: start / stop / status")
	os.Exit(2)
}
