//go:build windows

package tray

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"unsafe"

	"github.com/reflaxess123/fuflogon/core"
	"github.com/reflaxess123/fuflogon/platform"
)

var (
	trayMu      sync.Mutex
	rootDir     string
	cfgName     string
	configs     []string
	trayRunning bool
	trayStatus  = "idle"
	xrayVersion string

	hostHWND     syscall.Handle
	hiconIdle    syscall.Handle
	hiconRunning syscall.Handle
)

// hostWndProc is the window procedure for the hidden host window.
func hostWndProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmNotifyIcon:
		lo := lParam & 0xFFFF
		switch lo {
		case wmLButtonUp:
			togglePopup()
		case wmRButtonUp:
			showContextMenu(hwnd)
		}
		return 0

	case wmCommand:
		id := wParam & 0xFFFF
		switch id {
		case cmdStart:
			go trayStart()
		case cmdStop:
			go trayStop()
		case cmdRestart:
			go trayRestart()
		case cmdQuit:
			go doQuit()
		}
		return 0

	case wmDestroy:
		postQuitMessage(0)
		return 0
	}
	return defWindowProc(hwnd, msg, wParam, lParam)
}

// RunTray is the entry point for tray mode (no CLI args).
func RunTray() {
	if !platform.EnsureAdmin() {
		_ = platform.RelaunchAsAdmin()
		return
	}

	rootDir = core.GetRootDir()
	configs = core.ScanConfigs(rootDir)
	if len(configs) == 0 {
		fmt.Fprintln(os.Stderr, "No config*.json files found in "+rootDir)
		os.Exit(1)
	}
	cfgName = core.PickDefaultConfig(configs)

	// Auto-download xray if missing
	xrayBin := filepath.Join(rootDir, core.XrayCandidates()[0])
	if !core.FileExists(xrayBin) {
		core.Logf("[INFO] xray binary not found, auto-downloading...")
		if err := core.DownloadXray(rootDir, func(s string) { core.Logf("[DOWNLOAD] %s", s) }); err != nil {
			core.Logf("[ERROR] auto-download xray: %v", err)
		}
	}

	xrayVersion = core.GetXrayVersion(rootDir)

	hiconIdle = icoToHICON(iconIdle)
	hiconRunning = icoToHICON(iconRunning)
	if hiconIdle == 0 {
		hiconIdle = hiconRunning
	}
	if hiconRunning == 0 {
		hiconRunning = hiconIdle
	}

	hInst := getModuleHandle()

	// Register host window class
	wc := wndClassEx{
		cbSize:      uint32(unsafe.Sizeof(wndClassEx{})),
		lpfnWndProc: syscall.NewCallback(hostWndProc),
		hInstance:   hInst,
	}
	cls, _ := syscall.UTF16PtrFromString("FuflogonHost")
	wc.lpszClassName = cls
	registerClassEx(&wc)

	// Create hidden 0×0 host window
	hostHWND = createWindowEx(
		0,
		"FuflogonHost", "",
		wsOverlapped,
		0, 0, 0, 0,
		0, 0, hInst,
	)

	trayAdd(hostHWND, hiconIdle, "Fuflogon VPN: idle")

	initPopupWindow(hInst)

	// Check if xray is already running
	if st, err := core.LoadState(rootDir); err == nil && platform.IsProcessRunning(st.XrayPID) {
		setTrayRunning(true)
		go trayCheckConnectivity()
	} else {
		go trayStart()
	}

	runMessageLoop()
}

func setTrayRunning(v bool) {
	trayMu.Lock()
	trayRunning = v
	trayMu.Unlock()
	if v {
		trayUpdate(hostHWND, hiconRunning, "Fuflogon VPN: running ["+filepath.Base(cfgName)+"]")
	} else {
		trayUpdate(hostHWND, hiconIdle, "Fuflogon VPN: idle ["+filepath.Base(cfgName)+"]")
	}
}

func trayStart() {
	updateStatus("starting...")
	if err := platform.Start(rootDir, filepath.Base(cfgName)); err != nil {
		msg := core.Truncate(err.Error(), 60)
		updateStatus("error: " + msg)
		showError(fmt.Sprintf("start failed:\n%v\n\nSee Logs/xray-launcher.log for details.", err))
		return
	}
	setTrayRunning(true)
	updateStatus("running")
	go trayCheckConnectivity()
}

func trayStop() {
	updateStatus("stopping...")
	if err := platform.Stop(rootDir); err != nil {
		updateStatus("error")
		return
	}
	setTrayRunning(false)
	updateStatus("idle")
}

func trayRestart() {
	updateStatus("restarting...")
	_ = platform.Stop(rootDir)
	if err := platform.Start(rootDir, filepath.Base(cfgName)); err != nil {
		msg := core.Truncate(err.Error(), 60)
		updateStatus("error: " + msg)
		showError(fmt.Sprintf("restart failed:\n%v\n\nSee Logs/xray-launcher.log for details.", err))
		return
	}
	setTrayRunning(true)
	updateStatus("running")
	go trayCheckConnectivity()
}

func trayUpdateGeo() {
	updateStatus("updating geo...")
	if err := core.UpdateGeo(rootDir); err != nil {
		core.Logf("[ERROR] update geo: %v", err)
		showError(fmt.Sprintf("Update geo failed:\n%v", err))
		updateStatus("update failed")
		return
	}
	trayMu.Lock()
	running := trayRunning
	trayMu.Unlock()
	if running {
		updateStatus("running")
	} else {
		updateStatus("idle")
	}
}

func trayDownloadXray() {
	updateStatus("downloading xray...")
	if err := core.DownloadXray(rootDir, func(s string) { updateStatus(s) }); err != nil {
		core.Logf("[ERROR] download xray: %v", err)
		showError(fmt.Sprintf("Download xray failed:\n%v", err))
		updateStatus("download failed")
		return
	}
	xrayVersion = core.GetXrayVersion(rootDir)
	trayMu.Lock()
	running := trayRunning
	trayMu.Unlock()
	if running {
		updateStatus("running")
	} else {
		updateStatus("idle")
	}
}

func updateStatus(s string) {
	trayMu.Lock()
	trayStatus = s
	trayMu.Unlock()
	if popupHWND != 0 {
		refreshPopup()
	}
}

func doQuit() {
	go trayStop()
	trayRemove(hostHWND)
	postQuitMessage(0)
}
