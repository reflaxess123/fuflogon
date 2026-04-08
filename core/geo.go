package core

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

const geoRawBase = "https://github.com/runetfreedom/russia-v2ray-rules-dat/raw/release"

var geoUpdating atomic.Bool

// UpdateGeo downloads geoip.dat / geosite.dat directly from GitHub raw.
// Uses 8.8.8.8 DNS bypass. 3-minute timeout. Protected against parallel runs.
func UpdateGeo(rootDir string) error {
	if !geoUpdating.CompareAndSwap(false, true) {
		return fmt.Errorf("update already in progress")
	}
	defer geoUpdating.Store(false)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Use 8.8.8.8 directly — xray intercepts system DNS and during TUN operation
	// resolving github.com through it may fail.
	directDialer := &net.Dialer{
		Resolver: &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "udp", "8.8.8.8:53")
			},
		},
	}
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: directDialer.DialContext,
		},
	}

	files := []string{"geoip.dat", "geoip.dat.sha256sum", "geosite.dat", "geosite.dat.sha256sum"}
	for _, name := range files {
		url := geoRawBase + "/" + name
		Logf("[UPDATE] downloading %s ...", name)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("download %s: %w", name, err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return fmt.Errorf("download %s: HTTP %d", name, resp.StatusCode)
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		dst := filepath.Join(rootDir, name)
		if err := os.WriteFile(dst, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
		Logf("[UPDATE] %s — %d bytes OK", name, len(data))
	}

	Logf("[UPDATE] done")
	return nil
}
