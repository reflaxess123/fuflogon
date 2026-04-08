package core

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const xrayGitHubAPI = "https://api.github.com/repos/xtls/xray-core/releases/latest"

// xrayAssetName returns the zip asset name for the current platform.
func xrayAssetName() string {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "windows/amd64":
		return "Xray-windows-64.zip"
	case "windows/arm64":
		return "Xray-windows-arm64-v8a.zip"
	case "linux/amd64":
		return "Xray-linux-64.zip"
	case "linux/arm64":
		return "Xray-linux-arm64-v8a.zip"
	case "darwin/amd64":
		return "Xray-macos-64.zip"
	case "darwin/arm64":
		return "Xray-macos-arm64-unsigned.zip"
	default:
		return "Xray-linux-64.zip"
	}
}

// xrayBinaryName returns the binary name inside the zip.
func xrayBinaryName() string {
	if runtime.GOOS == "windows" {
		return "xray.exe"
	}
	return "xray"
}

// directHTTPClient creates an HTTP client with 8.8.8.8 DNS bypass (same as geo.go).
func directHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Resolver: &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "udp", "8.8.8.8:53")
			},
		},
	}
	return &http.Client{
		Transport: &http.Transport{DialContext: dialer.DialContext},
		Timeout:   timeout,
	}
}

// DownloadXray downloads the latest xray binary for the current platform
// into rootDir. progress is called with byte-level progress (may be nil).
//
// CAUTION: this writes to xray.exe in-place. Caller MUST stop any running
// xray process first, otherwise the write fails with sharing violation.
func DownloadXray(rootDir string, progress ProgressFn) error {
	if progress == nil {
		progress = func(Progress) {}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	client := directHTTPClient(5 * time.Minute)

	// 1. Get latest release info
	progress(Progress{Active: true, Stage: "Fetching xray release info", Step: 1, StepCount: 3})
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, xrayGitHubAPI, nil)
	req.Header.Set("User-Agent", "fuflogon-launcher/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch release info: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("github API: HTTP %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("parse release info: %w", err)
	}

	// 2. Find the right asset
	assetName := xrayAssetName()
	var downloadURL string
	for _, a := range release.Assets {
		if a.Name == assetName {
			downloadURL = a.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		return fmt.Errorf("asset %q not found in release %s", assetName, release.TagName)
	}

	// 3. Download zip with byte-level progress
	req2, _ := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	req2.Header.Set("User-Agent", "fuflogon-launcher/1.0")
	resp2, err := client.Do(req2)
	if err != nil {
		return fmt.Errorf("download zip: %w", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		return fmt.Errorf("download zip: HTTP %d", resp2.StatusCode)
	}

	pr := newProgressReader(resp2.Body, resp2.ContentLength,
		fmt.Sprintf("Downloading xray %s", release.TagName), assetName,
		2, 3, progress)
	zipData, err := io.ReadAll(pr)
	if err != nil {
		return fmt.Errorf("read zip: %w", err)
	}

	progress(Progress{Active: true, Stage: "Extracting xray", File: assetName, Step: 3, StepCount: 3})

	// 4. Extract binary from zip
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}

	binName := xrayBinaryName()
	var binData []byte
	for _, f := range zr.File {
		if f.Name == binName || strings.HasSuffix(f.Name, "/"+binName) {
			rc, err := f.Open()
			if err != nil {
				return fmt.Errorf("open %s in zip: %w", binName, err)
			}
			binData, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return fmt.Errorf("read %s from zip: %w", binName, err)
			}
			break
		}
	}
	if binData == nil {
		return fmt.Errorf("%s not found in zip", binName)
	}

	// 5. Write to rootDir. On Windows, if a process holds the file, this fails
	// with sharing violation — so we try removing it first.
	outPath := filepath.Join(rootDir, binName)
	if FileExists(outPath) {
		_ = os.Remove(outPath)
	}
	if err := os.WriteFile(outPath, binData, 0755); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}

	Logf("[UPDATE] xray %s downloaded (%d bytes)", release.TagName, len(binData))
	return nil
}

// GetXrayVersion runs xray --version and returns the version string.
// Returns empty string on error.
func GetXrayVersion(rootDir string) string {
	binName := xrayBinaryName()
	binPath := filepath.Join(rootDir, binName)
	if !FileExists(binPath) {
		return ""
	}
	out, err := exec.Command(binPath, "version").Output()
	if err != nil {
		return ""
	}
	// First line of output, e.g. "Xray 25.1.1 (Xray, Penetrates Everything.)"
	lines := strings.SplitN(string(out), "\n", 2)
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return ""
}
