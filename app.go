package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/reflaxess123/fuflogon/core"
	"github.com/reflaxess123/fuflogon/platform"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// AppStatus mirrors the React-side status enum.
type AppStatus string

const (
	StatusIdle       AppStatus = "idle"
	StatusStarting   AppStatus = "starting"
	StatusRunning    AppStatus = "running"
	StatusStopping   AppStatus = "stopping"
	StatusRestarting AppStatus = "restarting"
	StatusError      AppStatus = "error"
)

// State is the full UI state shipped to the frontend.
type State struct {
	Status           AppStatus                    `json:"status"`
	Message          string                       `json:"message"`
	Configs          []string                     `json:"configs"`
	SelectedConfig   string                       `json:"selectedConfig"`
	XrayVersion      string                       `json:"xrayVersion"`
	ConfigInfo       *core.ConfigInfo             `json:"configInfo,omitempty"`
	RuStatus         []int                        `json:"ruStatus"`
	BlockedStatus    []int                        `json:"blockedStatus"`
	RuServices       []string                     `json:"ruServices"`
	BlockedServices  []string                     `json:"blockedServices"`
	RuOutbounds      []core.ResolveOutboundResult `json:"ruOutbounds"`
	BlockedOutbounds []core.ResolveOutboundResult `json:"blockedOutbounds"`
	Checking         bool                         `json:"checking"`
	Progress         core.Progress                `json:"progress"`
}

var ruServices = []string{
	"ya.ru", "rzd.ru", "gosuslugi.ru", "avito.ru", "ozon.ru",
}

var blockedServices = []string{
	"telegram.org", "youtube.com", "anthropic.com", "openai.com",
}

// App is the Wails-bound application object.
type App struct {
	ctx context.Context

	mu               sync.Mutex
	rootDir          string
	status           AppStatus
	message          string
	configs          []string
	selectedConfig   string
	xrayVersion      string
	configInfo       *core.ConfigInfo
	ruStatus         []int
	blockedStatus    []int
	ruOutbounds      []core.ResolveOutboundResult
	blockedOutbounds []core.ResolveOutboundResult
	checking         bool
	progress         core.Progress
}

func NewApp() *App {
	return &App{
		status:           StatusIdle,
		ruStatus:         make([]int, len(ruServices)),
		blockedStatus:    make([]int, len(blockedServices)),
		ruOutbounds:      make([]core.ResolveOutboundResult, len(ruServices)),
		blockedOutbounds: make([]core.ResolveOutboundResult, len(blockedServices)),
	}
}

// startup is called by Wails after the window is created.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	a.rootDir = core.GetRootDir()
	core.InitLog(a.rootDir)
	core.Logf("=== fuflogon (wails) start ===")

	a.configs = core.ScanConfigs(a.rootDir)
	if len(a.configs) > 0 {
		a.selectedConfig = core.PickDefaultConfig(a.configs)
		a.loadConfigInfo()
	}
	a.xrayVersion = core.GetXrayVersion(a.rootDir)

	go a.logTickerLoop()
	go core.TailXrayLog(a.rootDir)

	// Bootstrap missing files (wintun.dll embedded, xray + geo downloaded
	// from the network) and only THEN start the tunnel.
	go a.bootstrapAndStart()
}

// bootstrapAndStart ensures wintun.dll, xray.exe and geo*.dat exist on disk
// (extracting / downloading anything missing) and then starts the tunnel.
// All progress is reported to the UI through a.onProgress.
func (a *App) bootstrapAndStart() {
	defer a.clearProgress()

	// 1. wintun.dll — embedded, instant.
	extractWintun(a.rootDir)

	// 2. xray binary — download if missing.
	xrayBin := filepath.Join(a.rootDir, core.XrayCandidates()[0])
	if !core.FileExists(xrayBin) {
		core.Logf("[BOOTSTRAP] xray binary missing, downloading...")
		a.setStatus(StatusStarting, "First-time setup: downloading xray...")
		if err := core.DownloadXray(a.rootDir, a.onProgress); err != nil {
			core.Logf("[ERROR] bootstrap xray: %v", err)
			a.setStatus(StatusError, "failed to download xray: "+err.Error())
			return
		}
		a.mu.Lock()
		a.xrayVersion = core.GetXrayVersion(a.rootDir)
		a.mu.Unlock()
	}

	// 3. geo databases — download if missing.
	geoip := filepath.Join(a.rootDir, "geoip.dat")
	geosite := filepath.Join(a.rootDir, "geosite.dat")
	if !core.FileExists(geoip) || !core.FileExists(geosite) {
		core.Logf("[BOOTSTRAP] geo files missing, downloading...")
		a.setStatus(StatusStarting, "First-time setup: downloading geo data...")
		if err := core.UpdateGeo(a.rootDir, a.onProgress); err != nil {
			core.Logf("[ERROR] bootstrap geo: %v", err)
			a.setStatus(StatusError, "failed to download geo: "+err.Error())
			return
		}
	}

	a.clearProgress()

	// 4. Now start the tunnel — either reattach to a running instance or
	//    spawn a fresh one.
	if st, err := core.LoadState(a.rootDir); err == nil && platform.IsProcessRunning(st.XrayPID) {
		a.setStatus(StatusRunning, "")
		go a.delayedConnectivityCheck()
	} else {
		a.Start()
	}
}

// logTickerLoop pushes the latest log buffer to the frontend whenever it grows.
// The frontend listens for the "logs" event and updates its Logs tab.
func (a *App) logTickerLoop() {
	var lastSize int
	for {
		time.Sleep(750 * time.Millisecond)
		if a.ctx == nil {
			continue
		}
		sz := core.LogBufferSize()
		if sz == lastSize {
			continue
		}
		lastSize = sz
		wruntime.EventsEmit(a.ctx, "logs", core.GetLogBuffer())
	}
}

// shutdown is called by Wails just before the app exits.
func (a *App) shutdown(ctx context.Context) {
	core.Logf("=== fuflogon shutdown ===")
	core.CloseLog()
}

// ---------------------------------------------------------------------------
// State helpers
// ---------------------------------------------------------------------------

func (a *App) snapshot() State {
	a.mu.Lock()
	defer a.mu.Unlock()
	ru := make([]int, len(a.ruStatus))
	copy(ru, a.ruStatus)
	bl := make([]int, len(a.blockedStatus))
	copy(bl, a.blockedStatus)
	ruOut := make([]core.ResolveOutboundResult, len(a.ruOutbounds))
	copy(ruOut, a.ruOutbounds)
	blOut := make([]core.ResolveOutboundResult, len(a.blockedOutbounds))
	copy(blOut, a.blockedOutbounds)
	return State{
		Status:           a.status,
		Message:          a.message,
		Configs:          append([]string(nil), a.configs...),
		SelectedConfig:   a.selectedConfig,
		XrayVersion:      a.xrayVersion,
		ConfigInfo:       a.configInfo,
		RuStatus:         ru,
		BlockedStatus:    bl,
		RuServices:       ruServices,
		BlockedServices:  blockedServices,
		RuOutbounds:      ruOut,
		BlockedOutbounds: blOut,
		Checking:         a.checking,
		Progress:         a.progress,
	}
}

// onProgress is the bridge between core.ProgressFn and the React frontend.
// Stores the latest snapshot in the App and emits a state event.
func (a *App) onProgress(p core.Progress) {
	a.mu.Lock()
	a.progress = p
	a.mu.Unlock()
	a.emit()
}

func (a *App) clearProgress() {
	a.mu.Lock()
	a.progress = core.Progress{}
	a.mu.Unlock()
	a.emit()
}

func (a *App) emit() {
	if a.ctx == nil {
		return
	}
	wruntime.EventsEmit(a.ctx, "state", a.snapshot())
}

func (a *App) setStatus(s AppStatus, msg string) {
	a.mu.Lock()
	a.status = s
	a.message = msg
	a.mu.Unlock()
	a.emit()
}

// ---------------------------------------------------------------------------
// Methods exposed to the frontend (auto-bound by Wails)
// ---------------------------------------------------------------------------

// GetState returns the current state. Used on app load.
func (a *App) GetState() State {
	return a.snapshot()
}

// GetLogs returns the in-memory log buffer (up to ~5000 lines).
func (a *App) GetLogs() []string {
	return core.GetLogBuffer()
}

// ClearLogs empties the in-memory log buffer.
func (a *App) ClearLogs() {
	core.ClearLogBuffer()
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "logs-cleared")
	}
}

// Quit cleanly stops the VPN tunnel, removes the tray icon and exits the
// process. Called from the Quit button in the UI.
func (a *App) Quit() {
	core.Logf("[APP] Quit requested")

	// 1. Stop xray synchronously so we don't leave routes/TUN behind.
	if a.rootDir != "" {
		if err := platform.Stop(a.rootDir); err != nil {
			core.Logf("[APP] stop error (continuing): %v", err)
		}
	}

	// 2. Remove the tray icon.
	trayQuit()

	// 3. Tell Wails to shut down (fires OnShutdown).
	if a.ctx != nil {
		wruntime.Quit(a.ctx)
	}

	// 4. Force exit after a short grace period so logs flush.
	go func() {
		time.Sleep(300 * time.Millisecond)
		core.Logf("[APP] os.Exit(0)")
		core.CloseLog()
		os.Exit(0)
	}()
}

// SelectConfig switches the active config (does not restart).
func (a *App) SelectConfig(cfg string) {
	a.mu.Lock()
	a.selectedConfig = cfg
	a.mu.Unlock()
	a.loadConfigInfo()
	a.emit()
}

// loadConfigInfo parses the currently selected config and stores the result.
// Also recomputes which outbound each test service resolves to.
// Caller must NOT hold a.mu.
func (a *App) loadConfigInfo() {
	cfg := a.currentConfig()
	if cfg == "" {
		return
	}
	info, err := core.ParseConfigInfo(cfg)
	if err != nil {
		core.Logf("[WARN] parse config info %s: %v", cfg, err)
		return
	}
	ruOut := make([]core.ResolveOutboundResult, len(ruServices))
	for i, h := range ruServices {
		ruOut[i] = core.ResolveOutbound(h, info)
	}
	blOut := make([]core.ResolveOutboundResult, len(blockedServices))
	for i, h := range blockedServices {
		blOut[i] = core.ResolveOutbound(h, info)
	}
	a.mu.Lock()
	a.configInfo = info
	a.ruOutbounds = ruOut
	a.blockedOutbounds = blOut
	a.mu.Unlock()
}

// Start launches xray with the currently selected config.
func (a *App) Start() {
	a.setStatus(StatusStarting, "Starting tunnel...")
	cfg := filepath.Base(a.currentConfig())
	if err := platform.Start(a.rootDir, cfg); err != nil {
		core.Logf("[ERROR] start: %v", err)
		a.setStatus(StatusError, err.Error())
		return
	}
	a.setStatus(StatusRunning, "")
	go a.delayedConnectivityCheck()
}

// delayedConnectivityCheck waits for the tunnel to settle before probing
// services. Routing tables, DNS and WebView2 may not be ready immediately
// after platform.Start returns, so testing too early gives false negatives.
func (a *App) delayedConnectivityCheck() {
	time.Sleep(3 * time.Second)
	a.CheckConnectivity()
}

// Stop stops xray and tears down the tunnel.
func (a *App) Stop() {
	a.setStatus(StatusStopping, "Stopping tunnel...")
	if err := platform.Stop(a.rootDir); err != nil {
		core.Logf("[ERROR] stop: %v", err)
		a.setStatus(StatusError, err.Error())
		return
	}
	a.setStatus(StatusIdle, "")
}

// Restart stops and starts xray.
func (a *App) Restart() {
	a.setStatus(StatusRestarting, "Restarting tunnel...")
	_ = platform.Stop(a.rootDir)
	cfg := filepath.Base(a.currentConfig())
	if err := platform.Start(a.rootDir, cfg); err != nil {
		core.Logf("[ERROR] restart: %v", err)
		a.setStatus(StatusError, err.Error())
		return
	}
	a.setStatus(StatusRunning, "")
	go a.delayedConnectivityCheck()
}

// UpdateGeo downloads fresh geoip.dat / geosite.dat with byte-level progress.
// If xray is running, the tunnel is stopped beforehand so the download goes
// through the real interface, not through the (potentially slow) VPS.
func (a *App) UpdateGeo() {
	wasRunning := a.currentStatus() == StatusRunning
	defer a.clearProgress()

	if wasRunning {
		a.setStatus(StatusStopping, "Stopping xray for fast download...")
		if err := platform.Stop(a.rootDir); err != nil {
			core.Logf("[WARN] stop before update geo failed: %v", err)
		}
	}

	a.setStatus(StatusRunning, "Updating geo databases...")
	if err := core.UpdateGeo(a.rootDir, a.onProgress); err != nil {
		core.Logf("[ERROR] update geo: %v", err)
		a.setStatus(StatusError, err.Error())
		return
	}

	if wasRunning {
		a.setStatus(StatusStarting, "Restarting tunnel...")
		cfg := filepath.Base(a.currentConfig())
		if err := platform.Start(a.rootDir, cfg); err != nil {
			core.Logf("[ERROR] restart after geo update: %v", err)
			a.setStatus(StatusError, err.Error())
			return
		}
		a.setStatus(StatusRunning, "")
		go a.delayedConnectivityCheck()
	} else {
		a.setStatus(StatusIdle, "")
	}
}

// DownloadXray downloads the latest xray binary from GitHub.
// If xray is currently running, it is stopped before the write and restarted
// after — otherwise the binary is locked and the write fails.
func (a *App) DownloadXray() {
	wasRunning := a.currentStatus() == StatusRunning
	defer a.clearProgress()

	if wasRunning {
		a.setStatus(StatusStopping, "Stopping xray for update...")
		if err := platform.Stop(a.rootDir); err != nil {
			core.Logf("[WARN] stop before update failed: %v", err)
		}
	}

	a.setStatus(StatusRunning, "Downloading xray...")
	if err := core.DownloadXray(a.rootDir, a.onProgress); err != nil {
		core.Logf("[ERROR] download xray: %v", err)
		a.setStatus(StatusError, err.Error())
		return
	}

	a.mu.Lock()
	a.xrayVersion = core.GetXrayVersion(a.rootDir)
	a.mu.Unlock()

	if wasRunning {
		a.setStatus(StatusStarting, "Restarting tunnel after update...")
		cfg := filepath.Base(a.currentConfig())
		if err := platform.Start(a.rootDir, cfg); err != nil {
			core.Logf("[ERROR] restart after update: %v", err)
			a.setStatus(StatusError, err.Error())
			return
		}
		a.setStatus(StatusRunning, "")
		go a.delayedConnectivityCheck()
	} else {
		a.setStatus(StatusIdle, "")
	}
}

// CheckConnectivity probes each service via HTTPS HEAD over IPv4 only.
// We avoid raw TCP dial because:
//  1. handshake success ≠ working site (TLS may still fail)
//  2. plain net.Dial picks IPv6 first if AAAA exists — leaks past the TUN
//  3. system DNS may stall through TUN — we use a custom HTTP client with
//     a tcp4-only dialer + 8s total timeout
//
// Each probe is logged so the user can see exactly what failed.
func (a *App) CheckConnectivity() {
	a.mu.Lock()
	if a.checking {
		a.mu.Unlock()
		return
	}
	a.checking = true
	for i := range a.ruStatus {
		a.ruStatus[i] = 3 // checking
	}
	for i := range a.blockedStatus {
		a.blockedStatus[i] = 3
	}
	a.mu.Unlock()
	a.emit()

	core.Logf("[CHECK] starting connectivity probe (%d ru + %d blocked)",
		len(ruServices), len(blockedServices))

	// Shared HTTP client: forces IPv4, 8s total timeout, no redirect follow
	// (we only care that the host responded at all).
	dialer := &net.Dialer{
		Timeout:   6 * time.Second,
		KeepAlive: 0,
	}
	client := &http.Client{
		Timeout: 8 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				// Force IPv4 — bypass IPv6 leak past the TUN.
				return dialer.DialContext(ctx, "tcp4", addr)
			},
			TLSHandshakeTimeout:   6 * time.Second,
			ResponseHeaderTimeout: 6 * time.Second,
			DisableKeepAlives:     true,
			ForceAttemptHTTP2:     false,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	var wg sync.WaitGroup
	check := func(host string, idx int, target *[]int, kind string) {
		defer wg.Done()
		url := "https://" + host
		req, _ := http.NewRequest(http.MethodHead, url, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 fuflogon/connectivity-check")
		start := time.Now()
		resp, err := client.Do(req)
		elapsed := time.Since(start)

		if err != nil {
			core.Logf("[CHECK] %-7s %-20s FAIL (%4dms) %v", kind, host, elapsed.Milliseconds(), err)
			a.mu.Lock()
			(*target)[idx] = 2
			a.mu.Unlock()
			return
		}
		resp.Body.Close()
		// Any HTTP response counts as "reachable" — even 4xx/5xx mean the
		// server is alive and responsive.
		core.Logf("[CHECK] %-7s %-20s OK   (%4dms) status=%d", kind, host, elapsed.Milliseconds(), resp.StatusCode)
		a.mu.Lock()
		(*target)[idx] = 1
		a.mu.Unlock()
	}

	for i, h := range ruServices {
		wg.Add(1)
		go check(h, i, &a.ruStatus, "RU")
	}
	for i, h := range blockedServices {
		wg.Add(1)
		go check(h, i, &a.blockedStatus, "BLOCKED")
	}
	wg.Wait()

	core.Logf("[CHECK] done")

	a.mu.Lock()
	a.checking = false
	a.mu.Unlock()
	a.emit()
}

// ---------------------------------------------------------------------------
// Private helpers
// ---------------------------------------------------------------------------

func (a *App) currentConfig() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.selectedConfig
}

func (a *App) currentStatus() AppStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.status
}
