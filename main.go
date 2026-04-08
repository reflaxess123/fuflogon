package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/reflaxess123/fuflogon/core"
	"github.com/reflaxess123/fuflogon/platform"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	defer core.RecoverPanic("main")

	// CLI mode (e.g. `fuflogon stop`) — bypass GUI
	if len(os.Args) >= 2 {
		runCLI()
		return
	}

	// Ensure admin before launching GUI on Windows
	if !platform.EnsureAdmin() {
		_ = platform.RelaunchAsAdmin()
		return
	}

	app := NewApp()

	// Start tray icon in background goroutine; it will request showing the
	// Wails window via runtime callbacks once Wails has initialized.
	go runTray(app)

	err := wails.Run(&options.App{
		Title:         "Fuflogon",
		Width:         460,
		Height:        820,
		DisableResize: true,
		Frameless:     true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 9, G: 14, B: 25, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		HideWindowOnClose: true,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			BackdropType:         windows.Mica,
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err.Error())
		os.Exit(1)
	}
}

func runCLI() {
	rootDir := core.GetRootDir()
	core.InitLog(rootDir)
	defer core.CloseLog()

	cmd := os.Args[1]
	switch cmd {
	case "start":
		cfgName := core.DefaultCfgName
		if len(os.Args) >= 3 {
			cfgName = os.Args[2]
		}
		if err := platform.Start(rootDir, cfgName); err != nil {
			fmt.Fprintln(os.Stderr, "start failed:", err)
			os.Exit(1)
		}
	case "stop":
		if err := platform.Stop(rootDir); err != nil {
			fmt.Fprintln(os.Stderr, "stop failed:", err)
			os.Exit(1)
		}
	case "status":
		platform.Status(rootDir)
	case "update-geo":
		if err := core.UpdateGeo(rootDir, cliProgress); err != nil {
			fmt.Fprintln(os.Stderr, "update geo failed:", err)
			os.Exit(1)
		}
	case "download-xray":
		if err := core.DownloadXray(rootDir, cliProgress); err != nil {
			fmt.Fprintln(os.Stderr, "download xray failed:", err)
			os.Exit(1)
		}
	default:
		fmt.Println(`fuflogon — VPN launcher

Usage:
  fuflogon                          GUI mode
  fuflogon start [config.json]      start VPN
  fuflogon stop                     stop VPN
  fuflogon status                   show status
  fuflogon update-geo               update geoip.dat / geosite.dat
  fuflogon download-xray            download latest xray binary`)
	}
}

// cliProgress prints byte-level progress to stdout in CLI mode.
func cliProgress(p core.Progress) {
	if p.Total > 0 {
		pct := float64(p.Downloaded) * 100 / float64(p.Total)
		fmt.Printf("\r%s: %s — %.1f%% (%d/%d)",
			p.Stage, p.File, pct, p.Downloaded, p.Total)
	} else {
		fmt.Printf("\r%s: %s — %d bytes", p.Stage, p.File, p.Downloaded)
	}
	if p.Total > 0 && p.Downloaded >= p.Total {
		fmt.Println()
	}
}
