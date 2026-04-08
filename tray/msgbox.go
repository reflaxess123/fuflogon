//go:build windows

package tray

import (
	"syscall"
	"unsafe"
)

var (
	user32      = syscall.NewLazyDLL("user32.dll")
	procMsgBoxW = user32.NewProc("MessageBoxW")
)

const (
	mbOK       = 0x00000000
	mbIconErr  = 0x00000010
	mbIconWarn = 0x00000030
	mbIconInfo = 0x00000040
)

// showError shows a MessageBox with an error icon (Windows).
func showError(msg string) {
	title, _ := syscall.UTF16PtrFromString("fuflogon error")
	body, _ := syscall.UTF16PtrFromString(msg)
	procMsgBoxW.Call(0,
		uintptr(unsafe.Pointer(body)),
		uintptr(unsafe.Pointer(title)),
		uintptr(mbOK|mbIconErr))
}

// showInfo shows an informational MessageBox.
func showInfo(msg string) {
	title, _ := syscall.UTF16PtrFromString("fuflogon")
	body, _ := syscall.UTF16PtrFromString(msg)
	procMsgBoxW.Call(0,
		uintptr(unsafe.Pointer(body)),
		uintptr(unsafe.Pointer(title)),
		uintptr(mbOK|mbIconInfo))
}

// ShowError is the exported version.
func ShowError(msg string) { showError(msg) }

// ShowInfo is the exported version.
func ShowInfo(msg string) { showInfo(msg) }
