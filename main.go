package main

import (
	"fmt"
	"os"

	"github.com/reflaxess123/fuflogon/core"
	"github.com/reflaxess123/fuflogon/platform"
	"github.com/reflaxess123/fuflogon/tray"
)

func main() {
	defer core.RecoverPanic("main")
	defer core.CloseLog()

	if len(os.Args) >= 2 {
		platform.AttachParentConsole()
	}

	rootDir := core.GetRootDir()
	core.InitLog(rootDir)
	core.Logf("=== fuflogon start: args=%v ===", os.Args)

	if len(os.Args) < 2 {
		tray.RunTray()
		return
	}

	cmd := os.Args[1]
	core.Logf("rootDir=%s cmd=%s", rootDir, cmd)

	switch cmd {
	case "start":
		cfgName := core.DefaultCfgName
		if len(os.Args) >= 3 {
			cfgName = os.Args[2]
		}
		if err := platform.Start(rootDir, cfgName); err != nil {
			core.Logf("[ERROR] start: %v", err)
			tray.ShowError(fmt.Sprintf("start failed:\n%v\n\nSee Logs/xray-launcher.log for details.", err))
			os.Exit(1)
		}
	case "stop":
		if err := platform.Stop(rootDir); err != nil {
			core.Logf("[ERROR] stop: %v", err)
			tray.ShowError(fmt.Sprintf("stop failed:\n%v", err))
			os.Exit(1)
		}
	case "status":
		platform.Status(rootDir)
	case "update-geo":
		if err := core.UpdateGeo(rootDir); err != nil {
			core.Logf("[ERROR] update geo: %v", err)
			tray.ShowError(fmt.Sprintf("update geo failed:\n%v", err))
			os.Exit(1)
		}
	case "download-xray":
		if err := core.DownloadXray(rootDir, func(s string) { fmt.Println(s) }); err != nil {
			core.Logf("[ERROR] download xray: %v", err)
			fmt.Fprintln(os.Stderr, "download xray failed:", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Print(`fuflogon — Windows VPN launcher for xray-core

Usage:
  fuflogon                          — tray mode (Windows)
  fuflogon start [config.json]      — start VPN
  fuflogon stop                     — stop VPN
  fuflogon status                   — show status
  fuflogon update-geo               — update geoip.dat / geosite.dat
  fuflogon download-xray            — download latest xray binary

All runtime files should be in the same directory as the executable.
Admin rights required on Windows.
`)
}
