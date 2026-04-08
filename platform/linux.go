//go:build linux

package platform

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
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

// RelaunchAsAdmin on Linux just returns an error with sudo instructions.
func RelaunchAsAdmin() error {
	exe, _ := os.Executable()
	args := strings.Join(os.Args[1:], " ")
	return fmt.Errorf("need root: sudo %s %s", exe, args)
}

// AttachParentConsole is a no-op on Linux.
func AttachParentConsole() {}

// FlushDNS flushes DNS cache on Linux.
func FlushDNS() {
	runHidden("resolvectl", "flush-caches")
}

// isProcessAlive uses syscall.Kill(pid, 0) to probe the process.
func isProcessAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

func detectDefaultInterface() (string, error) {
	out, err := runHidden("ip", "route", "show", "default")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		for i, f := range fields {
			if f == "dev" && i+1 < len(fields) {
				return fields[i+1], nil
			}
		}
	}
	return "", fmt.Errorf("no default route")
}

func getDefaultGateway() (string, error) {
	out, err := runHidden("ip", "route", "show", "default")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		for i, f := range fields {
			if f == "via" && i+1 < len(fields) {
				return fields[i+1], nil
			}
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

func waitForTUN(timeout time.Duration) (int, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		out, err := runHidden("ip", "link", "show", core.TunName)
		if err == nil && strings.Contains(out, core.TunName) {
			fields := strings.SplitN(out, ":", 3)
			if len(fields) >= 1 {
				if idx, err := strconv.Atoi(strings.TrimSpace(fields[0])); err == nil {
					return idx, nil
				}
			}
			return 0, nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return 0, fmt.Errorf("timeout waiting for %s", core.TunName)
}

func setupTUNAddress(name string, ifIndex int) error {
	if _, err := runHidden("ip", "addr", "add", fmt.Sprintf("%s/%d", core.TunIP, core.TunPrefix), "dev", name); err != nil {
		if !strings.Contains(err.Error(), "exit") {
			return fmt.Errorf("ip addr add: %w", err)
		}
	}
	if _, err := runHidden("ip", "link", "set", name, "up"); err != nil {
		return fmt.Errorf("ip link up: %w", err)
	}
	if _, err := runHidden("ip", "link", "set", name, "mtu", strconv.Itoa(core.TunMTU)); err != nil {
		fmt.Printf("[WARN] set mtu: %v\n", err)
	}
	return nil
}

func addHostRoute(ip, gateway, iface string) error {
	_, err := runHidden("ip", "route", "add", ip+"/32", "via", gateway, "dev", iface)
	return err
}

func delHostRoute(ip string) error {
	_, err := runHidden("ip", "route", "del", ip+"/32")
	return err
}

func setDefaultRouteViaTUN(name string, ifIndex int) error {
	runHidden("ip", "route", "del", "default")
	_, err := runHidden("ip", "route", "add", "default", "dev", name, "metric", "1")
	return err
}

func delDefaultRouteViaTUN(name string, ifIndex int) error {
	_, err := runHidden("ip", "route", "del", "default", "dev", name)
	return err
}

func restoreDefaultRoute(gateway, iface string) error {
	_, err := runHidden("ip", "route", "add", "default", "via", gateway, "dev", iface)
	return err
}

// Start implements the VPN start sequence on Linux.
func Start(rootDir, cfgName string) error {
	if !EnsureAdmin() {
		return RelaunchAsAdmin()
	}

	binPath, err := core.FindXrayBinary(rootDir)
	if err != nil {
		return err
	}

	cfgPath := fmt.Sprintf("%s/%s", rootDir, cfgName)
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
		return fmt.Errorf("TUN adapter %s did not appear: %w", core.TunName, err)
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

// Stop undoes what Start did on Linux.
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

// Status prints the current VPN status on Linux.
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
