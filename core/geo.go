package core

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// jsDelivr CDN — global CDN serving GitHub repositories. Faster and not
// rate-limited like raw.githubusercontent.com.
const geoRawBase = "https://cdn.jsdelivr.net/gh/runetfreedom/russia-v2ray-rules-dat@release"

// Fallback if jsDelivr is unreachable from the user's network.
const geoFallbackBase = "https://raw.githubusercontent.com/runetfreedom/russia-v2ray-rules-dat/release"

var geoUpdating atomic.Bool

// UpdateGeo downloads geoip.dat / geosite.dat in parallel directly from
// raw.githubusercontent.com. Uses 8.8.8.8 DNS bypass. Reports aggregate
// byte-level progress through the callback.
func UpdateGeo(rootDir string, progress ProgressFn) error {
	if !geoUpdating.CompareAndSwap(false, true) {
		return fmt.Errorf("update already in progress")
	}
	defer geoUpdating.Store(false)

	if progress == nil {
		progress = func(Progress) {}
	}

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
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	client := &http.Client{
		Transport: &http.Transport{
			DialContext:           directDialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          10,
			MaxConnsPerHost:       10,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	files := []string{"geoip.dat", "geoip.dat.sha256sum", "geosite.dat", "geosite.dat.sha256sum"}

	// First pass: HEAD request all files to learn total size for the global
	// progress bar.
	var totalSize int64
	sizes := make([]int64, len(files))
	{
		var sizeWg sync.WaitGroup
		var sizeMu sync.Mutex
		for i, name := range files {
			sizeWg.Add(1)
			go func(i int, name string) {
				defer sizeWg.Done()
				req, _ := http.NewRequestWithContext(ctx, http.MethodHead, geoRawBase+"/"+name, nil)
				resp, err := client.Do(req)
				if err != nil {
					return
				}
				resp.Body.Close()
				sizeMu.Lock()
				sizes[i] = resp.ContentLength
				totalSize += resp.ContentLength
				sizeMu.Unlock()
			}(i, name)
		}
		sizeWg.Wait()
	}

	Logf("[UPDATE] geo total size: %d bytes across %d files", totalSize, len(files))

	// Second pass: download all files in parallel, share a single atomic counter
	// for aggregate progress.
	var downloaded atomic.Int64
	var dlWg sync.WaitGroup
	errCh := make(chan error, len(files))

	// Periodic progress emitter — single goroutine, throttled.
	stopEmit := make(chan struct{})
	go func() {
		ticker := time.NewTicker(150 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopEmit:
				return
			case <-ticker.C:
				progress(Progress{
					Active:     true,
					Stage:      "Updating geo data",
					File:       fmt.Sprintf("%d files in parallel", len(files)),
					Downloaded: downloaded.Load(),
					Total:      totalSize,
				})
			}
		}
	}()

	for i, name := range files {
		dlWg.Add(1)
		go func(i int, name string) {
			defer dlWg.Done()
			url := geoRawBase + "/" + name
			Logf("[UPDATE] downloading %s ...", name)
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			resp, err := client.Do(req)
			if err != nil {
				errCh <- fmt.Errorf("download %s: %w", name, err)
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				errCh <- fmt.Errorf("download %s: HTTP %d", name, resp.StatusCode)
				return
			}

			// Stream to disk while incrementing the shared atomic counter.
			dst := filepath.Join(rootDir, name)
			tmp := dst + ".tmp"
			f, err := os.Create(tmp)
			if err != nil {
				errCh <- fmt.Errorf("create %s: %w", tmp, err)
				return
			}

			buf := make([]byte, 64*1024)
			var fileBytes int64
			for {
				n, rerr := resp.Body.Read(buf)
				if n > 0 {
					if _, werr := f.Write(buf[:n]); werr != nil {
						f.Close()
						os.Remove(tmp)
						errCh <- fmt.Errorf("write %s: %w", dst, werr)
						return
					}
					fileBytes += int64(n)
					downloaded.Add(int64(n))
				}
				if rerr == io.EOF {
					break
				}
				if rerr != nil {
					f.Close()
					os.Remove(tmp)
					errCh <- fmt.Errorf("read %s: %w", name, rerr)
					return
				}
			}
			if err := f.Close(); err != nil {
				os.Remove(tmp)
				errCh <- fmt.Errorf("close %s: %w", tmp, err)
				return
			}
			if err := os.Rename(tmp, dst); err != nil {
				errCh <- fmt.Errorf("rename %s: %w", tmp, err)
				return
			}
			Logf("[UPDATE] %s — %d bytes OK", name, fileBytes)
		}(i, name)
	}

	dlWg.Wait()
	close(stopEmit)
	close(errCh)

	for err := range errCh {
		if err != nil {
			return err
		}
	}

	// Final 100% emit so the UI doesn't get stuck at 99%.
	progress(Progress{
		Active:     true,
		Stage:      "Updating geo data",
		File:       "done",
		Downloaded: totalSize,
		Total:      totalSize,
	})

	Logf("[UPDATE] done")
	return nil
}
