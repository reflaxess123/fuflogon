//go:build windows

package tray

import _ "embed"

//go:embed assets/icon_idle.ico
var iconIdle []byte

//go:embed assets/icon_running.ico
var iconRunning []byte
