//go:build windows

package tray

import (
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var ruServices = []string{
	"vk.com",
	"yandex.ru",
	"mail.ru",
	"ok.ru",
	"gosuslugi.ru",
	"sber.ru",
	"avito.ru",
	"wildberries.ru",
	"ozon.ru",
	"kinopoisk.ru",
}

var blockedServices = []string{
	"youtube.com",
	"telegram.org",
	"instagram.com",
	"facebook.com",
	"x.com",
	"tiktok.com",
	"discord.com",
	"spotify.com",
	"reddit.com",
	"linkedin.com",
}

// 0 = unknown, 1 = OK, 2 = fail
var connRuStatus = make([]int, 10)
var connBlockedStatus = make([]int, 10)
var connChecking atomic.Bool

func trayCheckConnectivity() {
	if !connChecking.CompareAndSwap(false, true) {
		return
	}
	defer connChecking.Store(false)

	// Save previous status text so we can restore it
	trayMu.Lock()
	prevStatus := trayStatus
	trayMu.Unlock()

	updateStatus("checking...")

	// Reset all statuses
	for i := range connRuStatus {
		connRuStatus[i] = 0
	}
	for i := range connBlockedStatus {
		connBlockedStatus[i] = 0
	}
	refreshPopup()

	var wg sync.WaitGroup

	for i, h := range ruServices {
		wg.Add(1)
		go func(idx int, host string) {
			defer wg.Done()
			conn, err := net.DialTimeout("tcp", host+":443", 4*time.Second)
			if err == nil {
				conn.Close()
				connRuStatus[idx] = 1
			} else {
				connRuStatus[idx] = 2
			}
			refreshPopup()
		}(i, h)
	}

	for i, h := range blockedServices {
		wg.Add(1)
		go func(idx int, host string) {
			defer wg.Done()
			conn, err := net.DialTimeout("tcp", host+":443", 4*time.Second)
			if err == nil {
				conn.Close()
				connBlockedStatus[idx] = 1
			} else {
				connBlockedStatus[idx] = 2
			}
			refreshPopup()
		}(i, h)
	}

	wg.Wait()
	refreshPopup()

	// Restore previous status (running / idle / error etc.)
	updateStatus(prevStatus)
}
