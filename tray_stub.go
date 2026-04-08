//go:build !windows

package main

func runTray(app *App) {
	// no-op on non-Windows for now
}

func trayQuit() {
	// no-op on non-Windows for now
}
