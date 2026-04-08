//go:build windows

package platform

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/reflaxess123/fuflogon/core"
)

const createNoWindow = 0x08000000

// hideWindow returns SysProcAttr to start a subprocess with no visible window.
func hideWindow() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

// runHidden runs a command without a window and returns stdout+stderr.
func runHidden(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = hideWindow()
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	out := strings.TrimSpace(buf.String())
	if err != nil {
		core.Logf("[CMD] %s %v => ERROR: %v | output: %q", name, args, err, out)
	} else {
		core.Logf("[CMD] %s %v => OK | output: %q", name, args, out)
	}
	return out, err
}

// powershell runs a PowerShell command.
func powershell(script string) (string, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	cmd.SysProcAttr = hideWindow()
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	out := strings.TrimSpace(buf.String())
	if err != nil {
		core.Logf("[PS] %s => ERROR: %v | output: %q", script, err, out)
	} else {
		core.Logf("[PS] %s => %q", script, out)
	}
	return out, err
}

// EnsureAdmin checks if we're running as admin.
func EnsureAdmin() bool {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY, 2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0, &sid)
	if err != nil {
		return false
	}
	defer windows.FreeSid(sid)
	token := windows.Token(0)
	member, err := token.IsMember(sid)
	return err == nil && member
}

// RelaunchAsAdmin re-opens this process via UAC.
func RelaunchAsAdmin() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cwd, _ := os.Getwd()
	verb, _ := syscall.UTF16PtrFromString("runas")
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	cwdPtr, _ := syscall.UTF16PtrFromString(cwd)
	args := strings.Join(os.Args[1:], " ")
	argPtr, _ := syscall.UTF16PtrFromString(args)
	if err := windows.ShellExecute(0, verb, exePtr, argPtr, cwdPtr, 1); err != nil {
		return fmt.Errorf("UAC denied: %w", err)
	}
	os.Exit(0)
	return nil
}

// detectDefaultInterface finds the network adapter for the default route.
func detectDefaultInterface() (string, error) {
	out, err := powershell(`(Get-NetRoute -DestinationPrefix '0.0.0.0/0' -ErrorAction SilentlyContinue | Where-Object { $_.NextHop -ne '0.0.0.0' } | Sort-Object -Property RouteMetric, InterfaceMetric | Select-Object -First 1).InterfaceAlias`)
	if err != nil {
		return "", err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return "", fmt.Errorf("no default route found")
	}
	return out, nil
}

// getDefaultGateway returns the current default gateway IP.
func getDefaultGateway() (string, error) {
	out, err := powershell(`(Get-NetRoute -DestinationPrefix '0.0.0.0/0' -ErrorAction SilentlyContinue | Where-Object { $_.NextHop -ne '0.0.0.0' } | Sort-Object -Property RouteMetric, InterfaceMetric | Select-Object -First 1).NextHop`)
	if err != nil {
		return "", err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return "", fmt.Errorf("no default gateway")
	}
	return out, nil
}

// StartXrayDetached starts xray.exe in the background without a visible window.
func StartXrayDetached(binPath, cfgPath, workDir string) (int, error) {
	core.Logf("[INFO] starting xray: %s run -c %s (workDir=%s)", binPath, cfgPath, workDir)

	xrayLogPath := filepath.Join(core.LogsDir(workDir), "xray-error.log")
	xrayLog, err := os.OpenFile(xrayLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		core.Logf("[WARN] cannot open xray-error.log: %v — xray output will be lost", err)
	}

	cmd := exec.Command(binPath, "run", "-c", cfgPath)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "XRAY_LOCATION_ASSET="+workDir)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow | windows.DETACHED_PROCESS,
	}
	if xrayLog != nil {
		cmd.Stdout = xrayLog
		cmd.Stderr = xrayLog
	}
	if err := cmd.Start(); err != nil {
		if xrayLog != nil {
			xrayLog.Close()
		}
		return 0, err
	}
	core.Logf("[INFO] xray started, PID=%d, output => %s", cmd.Process.Pid, xrayLogPath)
	go func() {
		cmd.Wait()
		if xrayLog != nil {
			xrayLog.Close()
		}
	}()
	return cmd.Process.Pid, nil
}

// WaitForTUN waits for the TUN adapter xray0 to appear and returns its ifIndex.
func WaitForTUN(timeout time.Duration) (int, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		out, err := powershell(fmt.Sprintf(`(Get-NetAdapter -Name '%s' -ErrorAction SilentlyContinue).ifIndex`, core.TunName))
		if err == nil {
			out = strings.TrimSpace(out)
			if out != "" {
				if idx, err := strconv.Atoi(out); err == nil && idx > 0 {
					return idx, nil
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return 0, fmt.Errorf("timeout waiting for adapter %s", core.TunName)
}

// setupTUNAddress assigns IP and MTU to the TUN adapter.
func setupTUNAddress(name string, ifIndex int) error {
	if _, err := runHidden("netsh", "interface", "ip", "set", "address",
		"name="+name, "static", core.TunIP, core.TunNetmask); err != nil {
		return fmt.Errorf("netsh set address: %w", err)
	}
	if _, err := runHidden("netsh", "interface", "ipv4", "set", "interface",
		name, fmt.Sprintf("mtu=%d", core.TunMTU)); err != nil {
		fmt.Printf("[WARN] netsh set mtu: %v\n", err)
	}
	return nil
}

// addHostRoute adds a static host route to the given IP via gateway.
func addHostRoute(ip, gateway, iface string) error {
	_, err := runHidden("route", "add", ip, "mask", "255.255.255.255", gateway, "metric", "1")
	return err
}

func delHostRoute(ip string) error {
	_, err := runHidden("route", "delete", ip)
	return err
}

// setDefaultRouteViaTUN adds a default route via TUN with metric 1.
func setDefaultRouteViaTUN(name string, ifIndex int) error {
	_, err := runHidden("route", "add", "0.0.0.0", "mask", "0.0.0.0", "0.0.0.0",
		"if", strconv.Itoa(ifIndex), "metric", "1")
	return err
}

// delDefaultRouteViaTUN removes our default route from TUN.
func delDefaultRouteViaTUN(name string, ifIndex int) error {
	_, err := runHidden("route", "delete", "0.0.0.0", "mask", "0.0.0.0",
		"if", strconv.Itoa(ifIndex))
	return err
}

// restoreDefaultRoute restores the old default route.
func restoreDefaultRoute(gateway, iface string) error {
	out, _ := powershell(`(Get-NetRoute -DestinationPrefix '0.0.0.0/0' -ErrorAction SilentlyContinue | Where-Object { $_.NextHop -ne '0.0.0.0' }).Count`)
	if strings.TrimSpace(out) != "" && strings.TrimSpace(out) != "0" {
		return nil
	}
	_, err := runHidden("route", "add", "0.0.0.0", "mask", "0.0.0.0", gateway)
	return err
}

// blockIPv6 installs a blackhole route for ::/0 so that any IPv6 traffic
// fails immediately and Windows falls back to IPv4 (which is captured by
// our TUN). Without this, hosts with AAAA records leak through Wi-Fi
// directly past the VPN.
//
// We add the route on the loopback interface (ifIndex=1) with metric 1 so
// it wins over any router-advertised default route. Returns nil even on
// failure — IPv6 may already be disabled, that's fine.
func blockIPv6() {
	// netsh refuses ::/0 — use unique-local prefix coverage instead.
	// Easiest: use `route -6 ADD ::/0 ::1 IF 1 METRIC 1` equivalent via netsh.
	if _, err := runHidden("netsh", "interface", "ipv6", "add", "route",
		"::/0", "interface=1", "nexthop=::", "metric=1", "store=active"); err != nil {
		core.Logf("[INFO] IPv6 blackhole route not added (probably IPv6 disabled): %v", err)
		return
	}
	core.Logf("[INFO] IPv6 blackhole route ::/0 → loopback installed")
}

// unblockIPv6 removes the blackhole route added by blockIPv6.
func unblockIPv6() {
	if _, err := runHidden("netsh", "interface", "ipv6", "delete", "route",
		"::/0", "interface=1", "nexthop=::"); err != nil {
		core.Logf("[INFO] IPv6 blackhole route not removed (probably never added): %v", err)
		return
	}
	core.Logf("[INFO] IPv6 blackhole route removed")
}

// AttachParentConsole attaches stdout/stderr to the parent process's console.
func AttachParentConsole() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	attachConsole := kernel32.NewProc("AttachConsole")
	const attachParentProcess = ^uintptr(0) // (DWORD)-1
	ret, _, _ := attachConsole.Call(attachParentProcess)
	if ret == 0 {
		kernel32.NewProc("AllocConsole").Call()
	}
	conout, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if err == nil {
		os.Stdout = conout
		os.Stderr = conout
	}
	_ = unsafe.Sizeof(0)
}

// FlushDNS flushes the Windows DNS cache.
func FlushDNS() {
	runHidden("ipconfig", "/flushdns")
}

// isProcessAlive is the Windows-specific implementation using tasklist.
func isProcessAlive(pid int) bool {
	out, err := runHidden("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH")
	if err != nil {
		return false
	}
	return strings.Contains(out, strconv.Itoa(pid))
}

// Start implements the full VPN start sequence on Windows.
func Start(rootDir, cfgName string) error {
	if !EnsureAdmin() {
		return RelaunchAsAdmin()
	}

	core.Logf("[SYS] OS=%s ARCH=%s rootDir=%s", runtime.GOOS, runtime.GOARCH, rootDir)
	if entries, err := os.ReadDir(rootDir); err == nil {
		for _, e := range entries {
			if info, err2 := e.Info(); err2 == nil {
				core.Logf("[SYS] file: %-40s size=%d", e.Name(), info.Size())
			}
		}
	}

	if err := os.Chdir(rootDir); err != nil {
		return fmt.Errorf("chdir %s: %w", rootDir, err)
	}

	binPath, err := core.FindXrayBinary(rootDir)
	if err != nil {
		return err
	}
	core.Logf("[INFO] xray binary: %s", binPath)

	cfgPath := filepath.Join(rootDir, cfgName)
	if !core.FileExists(cfgPath) {
		return fmt.Errorf("config not found: %s", cfgPath)
	}
	core.Logf("[INFO] config: %s", cfgPath)

	if !core.FileExists(filepath.Join(rootDir, "geoip.dat")) {
		return fmt.Errorf("geoip.dat not found in %s (run update-geo first)", rootDir)
	}
	if !core.FileExists(filepath.Join(rootDir, "geosite.dat")) {
		return fmt.Errorf("geosite.dat not found in %s (run update-geo first)", rootDir)
	}
	if !core.FileExists(filepath.Join(rootDir, "wintun.dll")) {
		return fmt.Errorf("wintun.dll not found in %s (download from wintun.net)", rootDir)
	}

	StopExistingXray(rootDir)

	realIface, err := detectDefaultInterface()
	if err != nil {
		return fmt.Errorf("cannot detect default interface: %w", err)
	}
	core.Logf("[INFO] real interface: %s", realIface)

	oldGw, err := getDefaultGateway()
	if err != nil {
		return fmt.Errorf("cannot get default gateway: %w", err)
	}
	core.Logf("[INFO] old default gateway: %s", oldGw)

	vpsIPs, err := core.ExtractServerIPs(cfgPath)
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	if len(vpsIPs) == 0 {
		return fmt.Errorf("no server IPs found in config")
	}
	core.Logf("[INFO] VPS IPs from config: %v", vpsIPs)

	runtimeCfg := filepath.Join(rootDir, core.RuntimeCfgName)
	if err := core.WriteRuntimeConfig(cfgPath, runtimeCfg, realIface); err != nil {
		return fmt.Errorf("write runtime config: %w", err)
	}
	core.Logf("[INFO] runtime config: %s (interface=%s)", runtimeCfg, realIface)

	pid, err := StartXrayDetached(binPath, runtimeCfg, rootDir)
	if err != nil {
		return fmt.Errorf("start xray: %w", err)
	}
	core.Logf("[INFO] xray pid: %d", pid)
	os.WriteFile(filepath.Join(rootDir, core.PidFileName), []byte(fmt.Sprintf("%d", pid)), 0644)

	core.Logf("[INFO] waiting for %s adapter...", core.TunName)
	tunIfIndex, err := WaitForTUN(15 * time.Second)
	if err != nil {
		KillProcess(pid)
		return fmt.Errorf("TUN adapter %s did not appear: %w", core.TunName, err)
	}
	core.Logf("[OK] %s is up (ifIndex=%d)", core.TunName, tunIfIndex)

	if err := setupTUNAddress(core.TunName, tunIfIndex); err != nil {
		KillProcess(pid)
		return fmt.Errorf("setup TUN address: %w", err)
	}

	for _, ip := range vpsIPs {
		if err := addHostRoute(ip, oldGw, realIface); err != nil {
			core.Logf("[WARN] add host route %s via %s failed: %v", ip, oldGw, err)
		} else {
			core.Logf("[INFO] added static route: %s via %s", ip, oldGw)
		}
	}

	st := core.State{
		OldGateway: oldGw,
		RealIface:  realIface,
		VPSIPs:     vpsIPs,
		XrayPID:    pid,
		TunIfIndex: tunIfIndex,
		RuntimeCfg: runtimeCfg,
	}
	if err := core.SaveState(rootDir, st); err != nil {
		core.Logf("[WARN] save state: %v", err)
	}

	if err := setDefaultRouteViaTUN(core.TunName, tunIfIndex); err != nil {
		KillProcess(pid)
		return fmt.Errorf("set default route via TUN: %w", err)
	}

	// Block IPv6 leaks — without this, hosts with AAAA records bypass our
	// IPv4-only TUN tunnel via the upstream router's IPv6 default route.
	blockIPv6()

	FlushDNS()

	fmt.Println()
	fmt.Println("============================================================")
	fmt.Println(" FUFLOGON VPN IS UP")
	fmt.Printf("  OS:         %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("  Real iface: %s\n", realIface)
	fmt.Printf("  TUN:        %s (ifIndex=%d)\n", core.TunName, tunIfIndex)
	fmt.Printf("  Config:     %s\n", cfgName)
	fmt.Printf("  VPS:        %s\n", strings.Join(vpsIPs, ", "))
	fmt.Printf("  PID:        %d\n", pid)
	fmt.Println(" To stop:    fuflogon stop")
	fmt.Println("============================================================")
	return nil
}

// Stop undoes what Start did, in the correct order.
func Stop(rootDir string) error {
	if !EnsureAdmin() {
		return RelaunchAsAdmin()
	}

	st, err := core.LoadState(rootDir)
	if err != nil {
		core.Logf("[WARN] no state file, doing best-effort cleanup: %v", err)
	}

	// 1. Remove default route from TUN
	if st != nil {
		if err := delDefaultRouteViaTUN(core.TunName, st.TunIfIndex); err != nil {
			core.Logf("[WARN] delete default route via TUN: %v", err)
		}
		// 2. Restore old default route
		if st.OldGateway != "" && st.RealIface != "" {
			if err := restoreDefaultRoute(st.OldGateway, st.RealIface); err != nil {
				core.Logf("[WARN] restore default route: %v", err)
			}
		}
		// 3. Remove static routes to VPS
		for _, ip := range st.VPSIPs {
			if err := delHostRoute(ip); err != nil {
				core.Logf("[WARN] delete host route %s: %v", ip, err)
			}
		}
	}

	// 3.5. Remove IPv6 blackhole route (no-op if it wasn't installed)
	unblockIPv6()

	// 4. Kill xray
	pidPath := filepath.Join(rootDir, core.PidFileName)
	if data, err := os.ReadFile(pidPath); err == nil {
		var pid int
		fmt.Sscanf(string(data), "%d", &pid)
		if pid > 0 {
			if err := KillProcess(pid); err != nil {
				core.Logf("[WARN] kill pid %d: %v", pid, err)
			}
		}
	}
	StopExistingXray(rootDir)

	// 5. Clean up temp files
	if st != nil && st.RuntimeCfg != "" {
		os.Remove(st.RuntimeCfg)
	}
	os.Remove(filepath.Join(rootDir, core.RuntimeCfgName))
	os.Remove(filepath.Join(rootDir, core.PidFileName))
	core.DeleteState(rootDir)

	FlushDNS()

	fmt.Println("============================================================")
	fmt.Println(" FUFLOGON VPN IS DOWN")
	fmt.Println("============================================================")
	return nil
}

// Status prints the current VPN status.
func Status(rootDir string) {
	st, err := core.LoadState(rootDir)
	if err != nil {
		fmt.Println("Status: NOT RUNNING (no state file)")
		return
	}
	running := IsProcessRunning(st.XrayPID)
	if running {
		fmt.Println("Status: RUNNING")
		fmt.Printf("  PID:        %d\n", st.XrayPID)
		fmt.Printf("  Real iface: %s\n", st.RealIface)
		fmt.Printf("  Old gw:     %s\n", st.OldGateway)
		fmt.Printf("  VPS IPs:    %s\n", strings.Join(st.VPSIPs, ", "))
		fmt.Printf("  TUN:        %s (ifIndex=%d)\n", core.TunName, st.TunIfIndex)
	} else {
		fmt.Println("Status: STALE (state exists but xray not running)")
		fmt.Println("Run: fuflogon stop  — to clean up")
	}
}
