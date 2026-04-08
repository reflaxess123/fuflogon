//go:build windows

package tray

import (
	"encoding/binary"
	"syscall"
	"unsafe"
)

// ---------------------------------------------------------------------------
// Window / system messages
// ---------------------------------------------------------------------------

const (
	wmDestroy     = 0x0002
	wmPaint       = 0x000F
	wmClose       = 0x0010
	wmCommand     = 0x0111
	wmLButtonUp   = 0x0202
	wmRButtonUp   = 0x0205
	wmMouseMove   = 0x0200
	wmMouseLeave  = 0x02A3
	wmActivate    = 0x0006
	wmEraseBkgnd  = 0x0014
	wmCtlColorBtn = 0x0135
	wmSetFont     = 0x0030
	wmUser        = 0x0400
	wmNotifyIcon  = wmUser + 1
	wmRefreshUI   = wmUser + 2
	waInactive    = 0
)

// ---------------------------------------------------------------------------
// Window styles / show commands / set-pos flags
// ---------------------------------------------------------------------------

const (
	wsOverlapped   = 0
	wsPopup        = 0x80000000
	wsVisible      = 0x10000000
	wsChild        = 0x40000000
	wsClipChildren = 0x02000000
	wsExToolWindow = 0x80
	wsExTopMost    = 8
	csDropShadow   = 0x20000
	bsPushButton   = 0
	swHide         = 0
	swShow         = 5
	swpNoSize      = 1
	swpNomove      = 2
	swpNoZorder    = 4
	swpNoActivate  = 0x10
	swpShowWindow  = 0x40
	hwndTopmost    = ^uintptr(0)
)

// ---------------------------------------------------------------------------
// Tray (Shell_NotifyIcon) flags
// ---------------------------------------------------------------------------

const (
	nimAdd     = 0
	nimModify  = 1
	nimDelete  = 2
	nifMessage = 1
	nifIcon    = 2
	nifTip     = 4
)

// ---------------------------------------------------------------------------
// Menu flags
// ---------------------------------------------------------------------------

const (
	mfString       = 0
	mfSeparator    = 0x800
	mfGrayed       = 1
	tpmLeftAlign   = 0
	tpmBottomAlign = 0x20
	tpmReturnCmd   = 0x100
	tpmRightButton = 2
	tpmNoNotify    = 0x80
)

// ---------------------------------------------------------------------------
// GDI constants
// ---------------------------------------------------------------------------

const (
	transparent        = 1
	opaque             = 2
	nullBrush          = 5
	psSolid            = 0
	psNull             = 5
	fwNormal           = 400
	fwBold             = 700
	antialiasedQuality = 4
	defaultCharset     = 1
	ffDontCare         = 0
)

// ---------------------------------------------------------------------------
// System metrics
// ---------------------------------------------------------------------------

const (
	smCxScreen     = 0
	smCyScreen     = 1
	lrDefaultColor = 0
)

// ---------------------------------------------------------------------------
// DWM
// ---------------------------------------------------------------------------

const (
	dwmwaWindowCornerPreference = 33
	dwmwcpRound                 = 2
)

// ---------------------------------------------------------------------------
// DrawText / TrackMouseEvent
// ---------------------------------------------------------------------------

const (
	tmLeave      = 0x2
	dtLeft       = 0
	dtVCenter    = 4
	dtSingleLine = 0x20
	dtNoclip     = 0x100
	dtNoPrefix   = 0x800
	dtCenter     = 1
)

// ---------------------------------------------------------------------------
// Tray UID
// ---------------------------------------------------------------------------

const trayUID = 1

// ---------------------------------------------------------------------------
// Command IDs (context menu)
// ---------------------------------------------------------------------------

const (
	cmdQuit    = 9001
	cmdStart   = 9002
	cmdStop    = 9003
	cmdRestart = 9004
)

// ---------------------------------------------------------------------------
// Button IDs
// ---------------------------------------------------------------------------

const (
	btnIDClose       = 100
	btnIDStart       = 101
	btnIDStop        = 102
	btnIDRestart     = 103
	btnIDQuit        = 104
	btnIDUpdateGeo   = 105
	btnIDRefreshConn = 106
	btnIDUpdateXray  = 107
)

// ---------------------------------------------------------------------------
// GDI colors (COLORREF = 0x00BBGGRR)
// ---------------------------------------------------------------------------

const (
	clrHeaderBg   uint32 = 0x003B291E // #1E293B dark navy
	clrHeaderText uint32 = 0x00FFFFFF
	clrBodyBg     uint32 = 0x00FFFFFF
	clrStatusBg   uint32 = 0x00F9F5F1 // #F1F5F9
	clrSeparator  uint32 = 0x00F0E8E2 // #E2E8F0
	clrText       uint32 = 0x003B291E // #1E293B
	clrTextLight  uint32 = 0x008B7464 // #64748B
	clrGreen      uint32 = 0x005EC522 // #22C55E
	clrRed        uint32 = 0x004444EF // #EF4444
	clrGrey       uint32 = 0x00B8A394 // #94A3B8
	clrFooterBg   uint32 = 0x00F9F5F1
)

// ---------------------------------------------------------------------------
// Structs
// ---------------------------------------------------------------------------

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     syscall.Handle
	hIcon         syscall.Handle
	hCursor       syscall.Handle
	hbrBackground syscall.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       syscall.Handle
}

// notifyIconData mirrors NOTIFYICONDATAW.
type notifyIconData struct {
	cbSize           uint32
	hWnd             syscall.Handle // +4 pad inserted by Go
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            syscall.Handle // +4 pad inserted by Go
	szTip            [128]uint16
}

type w32Point struct{ x, y int32 }
type w32Rect struct{ left, top, right, bottom int32 }

type paintStruct struct {
	hdc         syscall.Handle
	fErase      int32
	rcPaint     w32Rect
	fRestore    int32
	fIncUpdate  int32
	rgbReserved [32]byte
}

type trackMouseEvent struct {
	cbSize      uint32
	dwFlags     uint32
	hwndTrack   syscall.Handle
	dwHoverTime uint32
}

type w32Msg struct {
	hwnd    syscall.Handle
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      w32Point
}

// ---------------------------------------------------------------------------
// DLL / proc vars
// ---------------------------------------------------------------------------

var (
	modU32  = syscall.NewLazyDLL("user32.dll")
	modGdi  = syscall.NewLazyDLL("gdi32.dll")
	modSh32 = syscall.NewLazyDLL("shell32.dll")
	modDwm  = syscall.NewLazyDLL("dwmapi.dll")
)

var (
	procRegisterClassExW_         = modU32.NewProc("RegisterClassExW")
	procCreateWindowExW_          = modU32.NewProc("CreateWindowExW")
	procDefWindowProcW_           = modU32.NewProc("DefWindowProcW")
	procShowWindow_               = modU32.NewProc("ShowWindow")
	procSetWindowPos_             = modU32.NewProc("SetWindowPos")
	procGetCursorPos_             = modU32.NewProc("GetCursorPos")
	procGetSystemMetrics_         = modU32.NewProc("GetSystemMetrics")
	procSetForegroundWindow_      = modU32.NewProc("SetForegroundWindow")
	procGetMessageW_              = modU32.NewProc("GetMessageW")
	procTranslateMessage_         = modU32.NewProc("TranslateMessage")
	procDispatchMessageW_         = modU32.NewProc("DispatchMessageW")
	procPostQuitMessage_          = modU32.NewProc("PostQuitMessage")
	procPostMessageW_             = modU32.NewProc("PostMessageW")
	procBeginPaint_               = modU32.NewProc("BeginPaint")
	procEndPaint_                 = modU32.NewProc("EndPaint")
	procInvalidateRect_           = modU32.NewProc("InvalidateRect")
	procGetClientRect_            = modU32.NewProc("GetClientRect")
	procGetWindowRect_            = modU32.NewProc("GetWindowRect")
	procCreatePopupMenu_          = modU32.NewProc("CreatePopupMenu")
	procAppendMenuW_              = modU32.NewProc("AppendMenuW")
	procTrackPopupMenu_           = modU32.NewProc("TrackPopupMenu")
	procDestroyMenu_              = modU32.NewProc("DestroyMenu")
	procGetModuleHandleW_         = modU32.NewProc("GetModuleHandleW")
	procFillRect_                 = modU32.NewProc("FillRect")
	procDrawTextW_                = modU32.NewProc("DrawTextW")
	procTrackMouseEvent_          = modU32.NewProc("TrackMouseEvent")
	procIsWindowVisible_          = modU32.NewProc("IsWindowVisible")
	procCreateIconFromResourceEx_ = modU32.NewProc("CreateIconFromResourceEx")
	procDestroyWindow_            = modU32.NewProc("DestroyWindow")
	procSendMessageW_             = modU32.NewProc("SendMessageW")
	procLoadCursorW_              = modU32.NewProc("LoadCursorW")
)

var (
	procCreateSolidBrush_ = modGdi.NewProc("CreateSolidBrush")
	procCreatePen_        = modGdi.NewProc("CreatePen")
	procDeleteObject_     = modGdi.NewProc("DeleteObject")
	procSelectObject_     = modGdi.NewProc("SelectObject")
	procEllipse_          = modGdi.NewProc("Ellipse")
	procSetBkMode_        = modGdi.NewProc("SetBkMode")
	procSetTextColor_     = modGdi.NewProc("SetTextColor")
	procTextOutW_         = modGdi.NewProc("TextOutW")
	procCreateFontW_      = modGdi.NewProc("CreateFontW")
	procGetStockObject_   = modGdi.NewProc("GetStockObject")
	procMoveToEx_         = modGdi.NewProc("MoveToEx")
	procLineTo_           = modGdi.NewProc("LineTo")
	procRectangle_        = modGdi.NewProc("Rectangle")
)

var procShellNotifyIconW_ = modSh32.NewProc("Shell_NotifyIconW")

var procDwmSetWindowAttribute_ = modDwm.NewProc("DwmSetWindowAttribute")

// ---------------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------------

func getModuleHandle() syscall.Handle {
	h, _, _ := procGetModuleHandleW_.Call(0)
	return syscall.Handle(h)
}

func registerClassEx(wc *wndClassEx) uint16 {
	r, _, _ := procRegisterClassExW_.Call(uintptr(unsafe.Pointer(wc)))
	return uint16(r)
}

func createWindowEx(exStyle uint32, className, windowName string, style uint32,
	x, y, w, h int32, parent, menu, instance syscall.Handle) syscall.Handle {

	cls, _ := syscall.UTF16PtrFromString(className)
	name, _ := syscall.UTF16PtrFromString(windowName)
	r, _, _ := procCreateWindowExW_.Call(
		uintptr(exStyle),
		uintptr(unsafe.Pointer(cls)),
		uintptr(unsafe.Pointer(name)),
		uintptr(style),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		uintptr(parent),
		uintptr(menu),
		uintptr(instance),
		0)
	return syscall.Handle(r)
}

func defWindowProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	r, _, _ := procDefWindowProcW_.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return r
}

func runMessageLoop() {
	var msg w32Msg
	for {
		r, _, _ := procGetMessageW_.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if r == 0 {
			break
		}
		procTranslateMessage_.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW_.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func w32Show(hwnd syscall.Handle, cmd int32) {
	procShowWindow_.Call(uintptr(hwnd), uintptr(cmd))
}

func w32SetPos(hwnd syscall.Handle, hwndAfter uintptr, x, y, w, h int32, flags uint32) {
	procSetWindowPos_.Call(uintptr(hwnd), hwndAfter,
		uintptr(x), uintptr(y), uintptr(w), uintptr(h), uintptr(flags))
}

func w32GetCursor() w32Point {
	var pt w32Point
	procGetCursorPos_.Call(uintptr(unsafe.Pointer(&pt)))
	return pt
}

func w32GetSysMetric(n int32) int32 {
	r, _, _ := procGetSystemMetrics_.Call(uintptr(n))
	return int32(r)
}

func w32ClientRect(hwnd syscall.Handle) w32Rect {
	var r w32Rect
	procGetClientRect_.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&r)))
	return r
}

func w32Invalidate(hwnd syscall.Handle) {
	procInvalidateRect_.Call(uintptr(hwnd), 0, 1)
}

func w32BeginPaint(hwnd syscall.Handle) (syscall.Handle, paintStruct) {
	var ps paintStruct
	r, _, _ := procBeginPaint_.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
	return syscall.Handle(r), ps
}

func w32EndPaint(hwnd syscall.Handle, ps *paintStruct) {
	procEndPaint_.Call(uintptr(hwnd), uintptr(unsafe.Pointer(ps)))
}

func w32IsVisible(hwnd syscall.Handle) bool {
	r, _, _ := procIsWindowVisible_.Call(uintptr(hwnd))
	return r != 0
}

func w32Post(hwnd syscall.Handle, msg uint32, w, l uintptr) {
	procPostMessageW_.Call(uintptr(hwnd), uintptr(msg), w, l)
}

func w32Send(hwnd syscall.Handle, msg uint32, w, l uintptr) uintptr {
	r, _, _ := procSendMessageW_.Call(uintptr(hwnd), uintptr(msg), w, l)
	return r
}

func w32FillRect(hdc syscall.Handle, r *w32Rect, brush syscall.Handle) {
	procFillRect_.Call(uintptr(hdc), uintptr(unsafe.Pointer(r)), uintptr(brush))
}

func w32DrawText(hdc syscall.Handle, text string, r *w32Rect, flags uint32) {
	p, _ := syscall.UTF16PtrFromString(text)
	procDrawTextW_.Call(uintptr(hdc),
		uintptr(unsafe.Pointer(p)), ^uintptr(0),
		uintptr(unsafe.Pointer(r)), uintptr(flags))
}

func postQuitMessage(code int32) {
	procPostQuitMessage_.Call(uintptr(code))
}

// ---------------------------------------------------------------------------
// GDI helpers
// ---------------------------------------------------------------------------

func gdiBrush(color uint32) syscall.Handle {
	r, _, _ := procCreateSolidBrush_.Call(uintptr(color))
	return syscall.Handle(r)
}

func gdiPen(style, w int32, color uint32) syscall.Handle {
	r, _, _ := procCreatePen_.Call(uintptr(style), uintptr(w), uintptr(color))
	return syscall.Handle(r)
}

func gdiDel(obj syscall.Handle) {
	procDeleteObject_.Call(uintptr(obj))
}

func gdiSel(hdc, obj syscall.Handle) syscall.Handle {
	r, _, _ := procSelectObject_.Call(uintptr(hdc), uintptr(obj))
	return syscall.Handle(r)
}

func gdiEllipse(hdc syscall.Handle, l, t, r, b int32) {
	procEllipse_.Call(uintptr(hdc), uintptr(l), uintptr(t), uintptr(r), uintptr(b))
}

func gdiSetBkMode(hdc syscall.Handle, mode int32) {
	procSetBkMode_.Call(uintptr(hdc), uintptr(mode))
}

func gdiSetTextColor(hdc syscall.Handle, color uint32) {
	procSetTextColor_.Call(uintptr(hdc), uintptr(color))
}

func gdiTextOut(hdc syscall.Handle, x, y int32, s string) {
	p, _ := syscall.UTF16PtrFromString(s)
	procTextOutW_.Call(uintptr(hdc), uintptr(x), uintptr(y),
		uintptr(unsafe.Pointer(p)), uintptr(len(s)))
}

func gdiCreateFont(height int32, face string, weight int32, quality byte) syscall.Handle {
	facePtr, _ := syscall.UTF16PtrFromString(face)
	r, _, _ := procCreateFontW_.Call(
		uintptr(height),
		0,
		0,
		0,
		uintptr(weight),
		0,
		0,
		0,
		uintptr(defaultCharset),
		0,
		0,
		uintptr(quality),
		uintptr(ffDontCare),
		uintptr(unsafe.Pointer(facePtr)),
	)
	return syscall.Handle(r)
}

func gdiLine(hdc syscall.Handle, x1, y1, x2, y2 int32) {
	procMoveToEx_.Call(uintptr(hdc), uintptr(x1), uintptr(y1), 0)
	procLineTo_.Call(uintptr(hdc), uintptr(x2), uintptr(y2))
}

func gdiRect(hdc syscall.Handle, l, t, r, b int32) {
	procRectangle_.Call(uintptr(hdc), uintptr(l), uintptr(t), uintptr(r), uintptr(b))
}

// ---------------------------------------------------------------------------
// Tray helpers
// ---------------------------------------------------------------------------

func trayNotify(hwnd syscall.Handle, icon syscall.Handle, tip string, msg uint32) {
	var nid notifyIconData
	nid.cbSize = uint32(unsafe.Sizeof(nid))
	nid.hWnd = hwnd
	nid.uID = trayUID
	nid.uFlags = nifMessage | nifIcon | nifTip
	nid.uCallbackMessage = wmNotifyIcon
	nid.hIcon = icon
	tipRunes, _ := syscall.UTF16FromString(tip)
	n := len(tipRunes)
	if n > 128 {
		n = 128
	}
	copy(nid.szTip[:], tipRunes[:n])
	procShellNotifyIconW_.Call(uintptr(msg), uintptr(unsafe.Pointer(&nid)))
}

func trayAdd(hwnd syscall.Handle, icon syscall.Handle, tip string) {
	trayNotify(hwnd, icon, tip, nimAdd)
}

func trayUpdate(hwnd syscall.Handle, icon syscall.Handle, tip string) {
	trayNotify(hwnd, icon, tip, nimModify)
}

func trayRemove(hwnd syscall.Handle) {
	var nid notifyIconData
	nid.cbSize = uint32(unsafe.Sizeof(nid))
	nid.hWnd = hwnd
	nid.uID = trayUID
	procShellNotifyIconW_.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
}

// ---------------------------------------------------------------------------
// DWM rounded corners
// ---------------------------------------------------------------------------

func dwmRound(hwnd syscall.Handle) {
	pref := uint32(dwmwcpRound)
	procDwmSetWindowAttribute_.Call(
		uintptr(hwnd),
		uintptr(dwmwaWindowCornerPreference),
		uintptr(unsafe.Pointer(&pref)),
		unsafe.Sizeof(pref),
	)
}

// ---------------------------------------------------------------------------
// ICO → HICON
// ---------------------------------------------------------------------------

func icoToHICON(data []byte) syscall.Handle {
	if len(data) < 6 {
		return 0
	}
	count := int(binary.LittleEndian.Uint16(data[4:6]))
	if count == 0 {
		return 0
	}

	type icoEntry struct {
		size   uint32
		offset uint32
		w, h   int
	}

	var best icoEntry
	for i := 0; i < count; i++ {
		base := 6 + i*16
		if base+16 > len(data) {
			break
		}
		w := int(data[base])
		h := int(data[base+1])
		if w == 0 {
			w = 256
		}
		if h == 0 {
			h = 256
		}
		size := binary.LittleEndian.Uint32(data[base+8 : base+12])
		offset := binary.LittleEndian.Uint32(data[base+12 : base+16])
		if w*h > best.w*best.h {
			best = icoEntry{size: size, offset: offset, w: w, h: h}
		}
	}

	if best.size == 0 || int(best.offset)+int(best.size) > len(data) {
		return 0
	}

	imgData := data[best.offset : best.offset+best.size]
	r, _, _ := procCreateIconFromResourceEx_.Call(
		uintptr(unsafe.Pointer(&imgData[0])),
		uintptr(best.size),
		1,          // fIcon = TRUE
		0x00030000, // dwVer = 3.0
		uintptr(best.w),
		uintptr(best.h),
		uintptr(lrDefaultColor),
	)
	return syscall.Handle(r)
}

// ---------------------------------------------------------------------------
// Context menu
// ---------------------------------------------------------------------------

func showContextMenu(hwnd syscall.Handle) {
	hMenu, _, _ := procCreatePopupMenu_.Call()
	if hMenu == 0 {
		return
	}

	appendMenuStr := func(id uint32, label string) {
		lbl, _ := syscall.UTF16PtrFromString(label)
		procAppendMenuW_.Call(hMenu, uintptr(mfString), uintptr(id),
			uintptr(unsafe.Pointer(lbl)))
	}

	appendMenuStr(cmdStart, "Start")
	appendMenuStr(cmdStop, "Stop")
	appendMenuStr(cmdRestart, "Restart")
	procAppendMenuW_.Call(hMenu, uintptr(mfSeparator), 0, 0)
	appendMenuStr(cmdQuit, "Quit")

	procSetForegroundWindow_.Call(uintptr(hwnd))

	pt := w32GetCursor()
	cmd, _, _ := procTrackPopupMenu_.Call(
		hMenu,
		uintptr(tpmReturnCmd|tpmRightButton|tpmNoNotify),
		uintptr(pt.x), uintptr(pt.y),
		0,
		uintptr(hwnd),
		0,
	)
	procDestroyMenu_.Call(hMenu)

	if cmd != 0 {
		w32Post(hwnd, wmCommand, cmd, 0)
	}
}
