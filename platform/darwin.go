//go:build darwin

package platform

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/reflaxess123/fuflogon/core"
)

func runHidden(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return strings.TrimSpace(buf.String()), err
}

// EnsureAdmin checks if we're running as root.
func EnsureAdmin() bool {
	return os.Geteuid() == 0
}

// RelaunchAsAdmin on macOS just returns an error with sudo instructions.
func RelaunchAsAdmin() error {
	exe, _ := os.Executable()
	args := strings.Join(os.Args[1:], " ")
	return fmt.Errorf("need root: sudo %s %s", exe, args)
}

// AttachParentConsole is a no-op on macOS.
func AttachParentConsole() {}

// FlushDNS flushes DNS cache on macOS.
func FlushDNS() {
	runHidden("dscacheutil", "-flushcache")
	runHidden("killall", "-HUP", "mDNSResponder")
}

// isProcessAlive uses syscall.Kill(pid, 0) to probe the process.
func isProcessAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

func detectDefaultInterface() (string, error) {
	out, err := runHidden("route", "-n", "get", "default")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "interface:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "interface:")), nil
		}
	}
	return "", fmt.Errorf("no default route")
}

func getDefaultGateway() (string, error) {
	out, err := runHidden("route", "-n", "get", "default")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "gateway:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "gateway:")), nil
		}
	}
	return "", fmt.Errorf("no default gateway")
}

func startXrayDetached(binPath, cfgPath, workDir string) (int, error) {
	cmd := exec.Command(binPath, "run", "-c", cfgPath)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "XRAY_LOCATION_ASSET="+workDir)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	go cmd.Wait()
	return cmd.Process.Pid, nil
}

func stopExistingXrayByName(rootDir string) {
	for _, name := range core.XrayCandidates() {
		runHidden("pkill", "-f", name+" run -c")
	}
}

// waitForTUN on macOS: xray creates utunN, look for any utun with our TUN IP.
func waitForTUN(timeout time.Duration) (int, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		out, err := runHidden("ifconfig", "-l")
		if err == nil {
			for _, name := range strings.Fields(out) {
				if strings.HasPrefix(name, "utun") {
					info, _ := runHidden("ifconfig", name)
					if strings.Contains(info, core.TunIP) {
						return 0, nil
					}
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return 0, fmt.Errorf("timeout waiting for utun adapter")
}

func setupTUNAddress(name string, ifIndex int) error {
	// macOS — IP is assigned by xray itself
	return nil
}

func addHostRoute(ip, gateway, iface string) error {
	_, err := runHidden("route", "add", "-host", ip, gateway)
	return err
}

func delHostRoute(ip string) error {
	_, err := runHidden("route", "delete", "-host", ip)
	return err
}

func setDefaultRouteViaTUN(name string, ifIndex int) error {
	out, _ := runHidden("ifconfig", "-l")
	var tunIface string
	for _, n := range strings.Fields(out) {
		if strings.HasPrefix(n, "utun") {
			info, _ := runHidden("ifconfig", n)
			if strings.Contains(info, core.TunIP) {
				tunIface = n
				break
			}
		}
	}
	if tunIface == "" {
		return fmt.Errorf("no utun adapter with %s", core.TunIP)
	}
	runHidden("route", "delete", "default")
	_, err := runHidden("route", "add", "default", "-interface", tunIface)
	return err
}

func delDefaultRouteViaTUN(name string, ifIndex int) error {
	_, err := runHidden("route", "delete", "default")
	return err
}

func restoreDefaultRoute(gateway, iface string) error {
	_, err := runHidden("route", "add", "default", gateway)
	return err
}

// Start implements the VPN start sequence on macOS.
func Start(rootDir, cfgName string) error {
	if !EnsureAdmin() {
		return RelaunchAsAdmin()
	}

	binPath, err := core.FindXrayBinary(rootDir)
	if err != nil {
		return err
	}

	cfgPath := rootDir + "/" + cfgName
	if !core.FileExists(cfgPath) {
		return fmt.Errorf("config not found: %s", cfgPath)
	}

	if !core.FileExists(rootDir + "/geoip.dat") {
		return fmt.Errorf("geoip.dat not found in %s", rootDir)
	}
	if !core.FileExists(rootDir + "/geosite.dat") {
		return fmt.Errorf("geosite.dat not found in %s", rootDir)
	}

	stopExistingXrayByName(rootDir)

	realIface, err := detectDefaultInterface()
	if err != nil {
		return fmt.Errorf("cannot detect default interface: %w", err)
	}

	oldGw, err := getDefaultGateway()
	if err != nil {
		return fmt.Errorf("cannot get default gateway: %w", err)
	}

	vpsIPs, err := core.ExtractServerIPs(cfgPath)
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	if len(vpsIPs) == 0 {
		return fmt.Errorf("no server IPs found in config")
	}

	runtimeCfg := rootDir + "/" + core.RuntimeCfgName
	if err := core.WriteRuntimeConfig(cfgPath, runtimeCfg, realIface); err != nil {
		return fmt.Errorf("write runtime config: %w", err)
	}

	pid, err := startXrayDetached(binPath, runtimeCfg, rootDir)
	if err != nil {
		return fmt.Errorf("start xray: %w", err)
	}
	os.WriteFile(rootDir+"/"+core.PidFileName, []byte(fmt.Sprintf("%d", pid)), 0644)

	tunIfIndex, err := waitForTUN(15 * time.Second)
	if err != nil {
		KillProcess(pid)
		return fmt.Errorf("TUN adapter did not appear: %w", err)
	}

	if err := setupTUNAddress(core.TunName, tunIfIndex); err != nil {
		KillProcess(pid)
		return fmt.Errorf("setup TUN address: %w", err)
	}

	for _, ip := range vpsIPs {
		if err := addHostRoute(ip, oldGw, realIface); err != nil {
			core.Logf("[WARN] add host route %s failed: %v", ip, err)
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

	FlushDNS()
	fmt.Println("VPN IS UP")
	return nil
}

// Stop undoes what Start did on macOS.
func Stop(rootDir string) error {
	if !EnsureAdmin() {
		return RelaunchAsAdmin()
	}

	st, err := core.LoadState(rootDir)
	if err != nil {
		core.Logf("[WARN] no state file, doing best-effort cleanup: %v", err)
	}

	if st != nil {
		if err := delDefaultRouteViaTUN(core.TunName, st.TunIfIndex); err != nil {
			core.Logf("[WARN] delete default route via TUN: %v", err)
		}
		if st.OldGateway != "" && st.RealIface != "" {
			if err := restoreDefaultRoute(st.OldGateway, st.RealIface); err != nil {
				core.Logf("[WARN] restore default route: %v", err)
			}
		}
		for _, ip := range st.VPSIPs {
			if err := delHostRoute(ip); err != nil {
				core.Logf("[WARN] delete host route %s: %v", ip, err)
			}
		}
	}

	pidPath := rootDir + "/" + core.PidFileName
	if data, err := os.ReadFile(pidPath); err == nil {
		var pid int
		fmt.Sscanf(string(data), "%d", &pid)
		if pid > 0 {
			if err := KillProcess(pid); err != nil {
				core.Logf("[WARN] kill pid %d: %v", pid, err)
			}
		}
	}
	stopExistingXrayByName(rootDir)

	if st != nil && st.RuntimeCfg != "" {
		os.Remove(st.RuntimeCfg)
	}
	os.Remove(rootDir + "/" + core.RuntimeCfgName)
	os.Remove(rootDir + "/" + core.PidFileName)
	core.DeleteState(rootDir)

	FlushDNS()
	fmt.Println("VPN IS DOWN")
	return nil
}

// Status prints the current VPN status on macOS.
func Status(rootDir string) {
	st, err := core.LoadState(rootDir)
	if err != nil {
		fmt.Println("Status: NOT RUNNING (no state file)")
		return
	}
	running := IsProcessRunning(st.XrayPID)
	if running {
		fmt.Println("Status: RUNNING")
		fmt.Printf("  PID:     %d\n", st.XrayPID)
		fmt.Printf("  Iface:   %s\n", st.RealIface)
		fmt.Printf("  VPS IPs: %s\n", strings.Join(st.VPSIPs, ", "))
	} else {
		fmt.Println("Status: STALE (state exists but xray not running)")
	}
}
