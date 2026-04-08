//go:build windows

package main

import (
	_ "embed"
	"time"

	"fyne.io/systray"
	"github.com/reflaxess123/fuflogon/core"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed assets/icon-idle.ico
var iconIdleBytes []byte

//go:embed assets/icon-running.ico
var iconRunningBytes []byte

// runTray sets up a minimal system tray icon — no context menu (those are
// flaky on Windows + UIPI), just a click handler that shows the main window.
func runTray(app *App) {
	systray.Run(func() { onTrayReady(app) }, func() {})
}

func onTrayReady(app *App) {
	systray.SetIcon(iconIdleBytes)
	systray.SetTitle("Fuflogon")
	systray.SetTooltip("Fuflogon: idle")

	// Left-click on the tray icon → show the main window.
	systray.SetOnTapped(func() {
		core.Logf("[TRAY] left click → show window")
		showWindow(app)
	})
	// Right-click also shows the window — we don't have a context menu and
	// don't want users wondering why nothing happens.
	systray.SetOnSecondaryTapped(func() {
		core.Logf("[TRAY] right click → show window")
		showWindow(app)
	})

	go trayWatchStatus(app)
}

// trayWatchStatus polls app status and updates the tray icon/tooltip.
func trayWatchStatus(app *App) {
	var lastStatus AppStatus = ""
	for {
		time.Sleep(800 * time.Millisecond)
		s := app.currentStatus()
		if s == lastStatus {
			continue
		}
		lastStatus = s
		if s == StatusRunning {
			systray.SetIcon(iconRunningBytes)
			systray.SetTooltip("Fuflogon: running")
		} else {
			systray.SetIcon(iconIdleBytes)
			systray.SetTooltip("Fuflogon: " + string(s))
		}
	}
}

func showWindow(app *App) {
	if app.ctx == nil {
		return
	}
	wruntime.WindowShow(app.ctx)
	wruntime.WindowUnminimise(app.ctx)
}

// trayQuit is called from App.Quit to remove the tray icon before exit.
func trayQuit() {
	systray.Quit()
}
