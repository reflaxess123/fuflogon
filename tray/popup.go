//go:build windows

package tray

import (
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/reflaxess123/fuflogon/core"
)

// ---------------------------------------------------------------------------
// Layout constants
// ---------------------------------------------------------------------------

const (
	popW = 360
	popH = 435

	secHeaderH  = 44
	secStatusH  = 56
	secBtnsH    = 46
	secSepH     = 1
	secConnHdrH = 36
	secConnH    = 200 // 10 rows × 20px
	secFooterH  = 52

	connRowH = 20
	connCircR = 7
)

// ---------------------------------------------------------------------------
// Package vars
// ---------------------------------------------------------------------------

var (
	popupHWND     syscall.Handle
	popupInstance syscall.Handle

	hwndBtnStart      syscall.Handle
	hwndBtnStop       syscall.Handle
	hwndBtnRestart    syscall.Handle
	hwndBtnQuit       syscall.Handle
	hwndBtnGeo        syscall.Handle
	hwndBtnUpdateXray syscall.Handle
	hwndBtnRefresh    syscall.Handle
	hwndBtnClose      syscall.Handle

	fontNormal syscall.Handle
	fontBold   syscall.Handle
	fontSmall  syscall.Handle
	fontHeader syscall.Handle
)

// ---------------------------------------------------------------------------
// popupWndProc — window procedure for the popup window
// ---------------------------------------------------------------------------

func popupWndProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmPaint:
		hdc, ps := w32BeginPaint(hwnd)
		drawPopup(hdc)
		w32EndPaint(hwnd, &ps)
		return 0

	case wmEraseBkgnd:
		return 1

	case wmActivate:
		if wParam == waInactive {
			w32Show(hwnd, swHide)
		}
		return 0

	case wmCommand:
		id := wParam & 0xFFFF
		switch id {
		case btnIDStart:
			go trayStart()
		case btnIDStop:
			go trayStop()
		case btnIDRestart:
			go trayRestart()
		case btnIDQuit:
			go doQuit()
		case btnIDUpdateGeo:
			go trayUpdateGeo()
		case btnIDUpdateXray:
			go trayDownloadXray()
		case btnIDRefreshConn:
			go trayCheckConnectivity()
		case btnIDClose:
			w32Show(hwnd, swHide)
		}
		return 0

	case wmRefreshUI:
		w32Invalidate(hwnd)
		return 0
	}
	return defWindowProc(hwnd, msg, wParam, lParam)
}

// ---------------------------------------------------------------------------
// initPopupWindow
// ---------------------------------------------------------------------------

func initPopupWindow(hInst syscall.Handle) {
	popupInstance = hInst

	wc := wndClassEx{
		cbSize:      uint32(unsafe.Sizeof(wndClassEx{})),
		style:       csDropShadow,
		lpfnWndProc: syscall.NewCallback(popupWndProc),
		hInstance:   hInst,
	}
	cls, _ := syscall.UTF16PtrFromString("FuflogonPopup")
	wc.lpszClassName = cls

	// Load default cursor
	cur, _, _ := procLoadCursorW_.Call(0, 32512) // IDC_ARROW
	wc.hCursor = syscall.Handle(cur)

	registerClassEx(&wc)

	popupHWND = createWindowEx(
		wsExToolWindow,
		"FuflogonPopup", "",
		wsPopup|wsClipChildren,
		0, 0, popW, popH,
		0, 0, hInst,
	)

	dwmRound(popupHWND)

	// Create fonts
	fontNormal = gdiCreateFont(-14, "Segoe UI", fwNormal, antialiasedQuality)
	fontBold = gdiCreateFont(-14, "Segoe UI", fwBold, antialiasedQuality)
	fontSmall = gdiCreateFont(-12, "Segoe UI", fwNormal, antialiasedQuality)
	fontHeader = gdiCreateFont(-16, "Segoe UI", 600, antialiasedQuality)

	// Create child buttons
	btnStyle := uint32(bsPushButton | wsChild | wsVisible)

	hwndBtnStart = createWindowEx(0, "BUTTON", "Start",
		btnStyle, 12, 108, 96, 30, popupHWND, syscall.Handle(btnIDStart), hInst)
	hwndBtnStop = createWindowEx(0, "BUTTON", "Stop",
		btnStyle, 116, 108, 96, 30, popupHWND, syscall.Handle(btnIDStop), hInst)
	hwndBtnRestart = createWindowEx(0, "BUTTON", "Restart",
		btnStyle, 220, 108, 96, 30, popupHWND, syscall.Handle(btnIDRestart), hInst)
	// Footer: [Update geo x=12,w=108] [Update xray x=128,w=108] [Quit x=284,w=64]
	hwndBtnGeo = createWindowEx(0, "BUTTON", "Update geo",
		btnStyle, 12, 391, 108, 32, popupHWND, syscall.Handle(btnIDUpdateGeo), hInst)
	hwndBtnUpdateXray = createWindowEx(0, "BUTTON", "Update xray",
		btnStyle, 128, 391, 108, 32, popupHWND, syscall.Handle(btnIDUpdateXray), hInst)
	hwndBtnQuit = createWindowEx(0, "BUTTON", "Quit",
		btnStyle, 284, 391, 64, 32, popupHWND, syscall.Handle(btnIDQuit), hInst)
	hwndBtnRefresh = createWindowEx(0, "BUTTON", "\u21BB",
		btnStyle, 320, 157, 28, 22, popupHWND, syscall.Handle(btnIDRefreshConn), hInst)
	hwndBtnClose = createWindowEx(0, "BUTTON", "\u2715",
		btnStyle, 324, 10, 24, 24, popupHWND, syscall.Handle(btnIDClose), hInst)

	// Set font on all buttons
	setFont := func(btn syscall.Handle) {
		w32Send(btn, wmSetFont, uintptr(fontNormal), 1)
	}
	setFont(hwndBtnStart)
	setFont(hwndBtnStop)
	setFont(hwndBtnRestart)
	setFont(hwndBtnQuit)
	setFont(hwndBtnGeo)
	setFont(hwndBtnUpdateXray)
	setFont(hwndBtnRefresh)
	setFont(hwndBtnClose)
}

// ---------------------------------------------------------------------------
// togglePopup / showPopup / refreshPopup
// ---------------------------------------------------------------------------

func togglePopup() {
	if w32IsVisible(popupHWND) {
		w32Show(popupHWND, swHide)
	} else {
		showPopup()
	}
}

func showPopup() {
	pt := w32GetCursor()
	sw := w32GetSysMetric(smCxScreen)
	sh := w32GetSysMetric(smCyScreen)

	x := pt.x - popW/2
	y := pt.y - popH - 8

	if x < 0 {
		x = 0
	}
	if x+popW > sw {
		x = sw - popW
	}
	if y < 0 {
		y = 0
	}
	if y+popH > sh {
		y = sh - popH
	}

	w32SetPos(popupHWND, hwndTopmost,
		x, y, 0, 0,
		swpNoSize|swpShowWindow|swpNoActivate)

	procSetForegroundWindow_.Call(uintptr(popupHWND))
}

func refreshPopup() {
	if popupHWND != 0 {
		w32Invalidate(popupHWND)
	}
}

// ---------------------------------------------------------------------------
// drawPopup — GDI painting
// ---------------------------------------------------------------------------

func drawPopup(hdc syscall.Handle) {
	gdiSetBkMode(hdc, transparent)

	// -----------------------------------------------------------------------
	// Section 1 — Header (y=0, h=44)
	// -----------------------------------------------------------------------
	hdrRect := w32Rect{0, 0, popW, secHeaderH}
	hdrBrush := gdiBrush(clrHeaderBg)
	w32FillRect(hdc, &hdrRect, hdrBrush)
	gdiDel(hdrBrush)

	oldFont := gdiSel(hdc, fontHeader)
	gdiSetTextColor(hdc, clrHeaderText)
	titleRect := w32Rect{16, 0, 310, secHeaderH}
	w32DrawText(hdc, "FUFLOGON VPN", &titleRect, dtVCenter|dtSingleLine|dtNoclip|dtNoPrefix)
	gdiSel(hdc, oldFont)

	// -----------------------------------------------------------------------
	// Section 2 — Status (y=44, h=56)
	// -----------------------------------------------------------------------
	statusY := int32(secHeaderH)
	statusRect := w32Rect{0, statusY, popW, statusY + secStatusH}
	statusBrush := gdiBrush(clrStatusBg)
	w32FillRect(hdc, &statusRect, statusBrush)
	gdiDel(statusBrush)

	// Separator line at top of status section
	sepBrush := gdiBrush(clrSeparator)
	sepRect := w32Rect{0, statusY, popW, statusY + 1}
	w32FillRect(hdc, &sepRect, sepBrush)
	gdiDel(sepBrush)

	// Status circle
	trayMu.Lock()
	status := trayStatus
	running := trayRunning
	ver := xrayVersion
	cfgBase := filepath.Base(cfgName)
	trayMu.Unlock()

	circX := int32(24)
	circY := statusY + 28
	circR := int32(10)

	var circColor uint32
	if running {
		circColor = clrGreen
	} else if len(status) > 5 && status[:5] == "error" {
		circColor = clrRed
	} else {
		circColor = clrGrey
	}

	circleBrush := gdiBrush(circColor)
	circlePen := gdiPen(psSolid, 1, circColor)
	oldBrush := gdiSel(hdc, circleBrush)
	oldPen := gdiSel(hdc, circlePen)
	gdiEllipse(hdc, circX-circR, circY-circR, circX+circR, circY+circR)
	gdiSel(hdc, oldBrush)
	gdiSel(hdc, oldPen)
	gdiDel(circleBrush)
	gdiDel(circlePen)

	// Status text
	oldFont = gdiSel(hdc, fontBold)
	gdiSetTextColor(hdc, clrText)
	statusTxtRect := w32Rect{44, statusY + 4, popW - 8, statusY + 22}
	w32DrawText(hdc, status, &statusTxtRect, dtLeft|dtSingleLine|dtNoclip|dtNoPrefix)
	gdiSel(hdc, oldFont)

	// Config name
	oldFont = gdiSel(hdc, fontSmall)
	gdiSetTextColor(hdc, clrTextLight)
	cfgTxtRect := w32Rect{44, statusY + 22, popW - 8, statusY + 38}
	w32DrawText(hdc, cfgBase, &cfgTxtRect, dtLeft|dtSingleLine|dtNoclip|dtNoPrefix)

	// Xray version
	if ver != "" {
		verTxtRect := w32Rect{44, statusY + 38, popW - 8, statusY + secStatusH}
		w32DrawText(hdc, core.Truncate(ver, 45), &verTxtRect, dtLeft|dtSingleLine|dtNoclip|dtNoPrefix)
	}
	gdiSel(hdc, oldFont)

	// -----------------------------------------------------------------------
	// Section 3 — Buttons background (y=100, h=46)
	// -----------------------------------------------------------------------
	btnsY := statusY + secStatusH
	btnsRect := w32Rect{0, btnsY, popW, btnsY + secBtnsH}
	btnsBrush := gdiBrush(clrBodyBg)
	w32FillRect(hdc, &btnsRect, btnsBrush)
	gdiDel(btnsBrush)

	// -----------------------------------------------------------------------
	// Section 4 — Separator (y=146, h=1)
	// -----------------------------------------------------------------------
	sep1Y := btnsY + secBtnsH
	sep1Rect := w32Rect{0, sep1Y, popW, sep1Y + secSepH}
	sep1Brush := gdiBrush(clrSeparator)
	w32FillRect(hdc, &sep1Rect, sep1Brush)
	gdiDel(sep1Brush)

	// -----------------------------------------------------------------------
	// Section 5 — Connectivity header (y=147, h=36)
	// -----------------------------------------------------------------------
	connHdrY := sep1Y + secSepH
	connHdrRect := w32Rect{0, connHdrY, popW, connHdrY + secConnHdrH}
	connHdrBrush := gdiBrush(clrBodyBg)
	w32FillRect(hdc, &connHdrRect, connHdrBrush)
	gdiDel(connHdrBrush)

	oldFont = gdiSel(hdc, fontBold)
	gdiSetTextColor(hdc, clrText)
	connLblRect := w32Rect{16, connHdrY + 8, 300, connHdrY + secConnHdrH}
	w32DrawText(hdc, "Connectivity", &connLblRect, dtLeft|dtSingleLine|dtNoclip|dtNoPrefix)
	gdiSel(hdc, oldFont)

	// -----------------------------------------------------------------------
	// Section 6 — Connectivity grid (y=183, h=200)
	// -----------------------------------------------------------------------
	gridY := connHdrY + secConnHdrH
	gridRect := w32Rect{0, gridY, popW, gridY + secConnH}
	gridBrush := gdiBrush(clrBodyBg)
	w32FillRect(hdc, &gridRect, gridBrush)
	gdiDel(gridBrush)

	oldFont = gdiSel(hdc, fontSmall)
	for i := 0; i < 10; i++ {
		rowY := gridY + int32(i)*connRowH

		// Left column — RU services
		{
			cx := int32(24)
			cy := rowY + connRowH/2

			var clr uint32
			switch connRuStatus[i] {
			case 1:
				clr = clrGreen
			case 2:
				clr = clrRed
			default:
				clr = clrGrey
			}
			rb := gdiBrush(clr)
			rp := gdiPen(psSolid, 1, clr)
			ob := gdiSel(hdc, rb)
			op := gdiSel(hdc, rp)
			gdiEllipse(hdc, cx-connCircR, cy-connCircR, cx+connCircR, cy+connCircR)
			gdiSel(hdc, ob)
			gdiSel(hdc, op)
			gdiDel(rb)
			gdiDel(rp)

			gdiSetTextColor(hdc, clrText)
			txtRect := w32Rect{36, rowY + 2, 176, rowY + connRowH}
			w32DrawText(hdc, ruServices[i], &txtRect, dtLeft|dtSingleLine|dtNoclip|dtNoPrefix)
		}

		// Right column — blocked services
		{
			cx := int32(204)
			cy := rowY + connRowH/2

			var clr uint32
			switch connBlockedStatus[i] {
			case 1:
				clr = clrGreen
			case 2:
				clr = clrRed
			default:
				clr = clrGrey
			}
			rb := gdiBrush(clr)
			rp := gdiPen(psSolid, 1, clr)
			ob := gdiSel(hdc, rb)
			op := gdiSel(hdc, rp)
			gdiEllipse(hdc, cx-connCircR, cy-connCircR, cx+connCircR, cy+connCircR)
			gdiSel(hdc, ob)
			gdiSel(hdc, op)
			gdiDel(rb)
			gdiDel(rp)

			gdiSetTextColor(hdc, clrText)
			txtRect := w32Rect{216, rowY + 2, popW - 4, rowY + connRowH}
			w32DrawText(hdc, blockedServices[i], &txtRect, dtLeft|dtSingleLine|dtNoclip|dtNoPrefix)
		}
	}
	gdiSel(hdc, oldFont)

	// -----------------------------------------------------------------------
	// Section 7 — Separator (y=383, h=1)
	// -----------------------------------------------------------------------
	sep2Y := gridY + secConnH
	sep2Rect := w32Rect{0, sep2Y, popW, sep2Y + secSepH}
	sep2Brush := gdiBrush(clrSeparator)
	w32FillRect(hdc, &sep2Rect, sep2Brush)
	gdiDel(sep2Brush)

	// -----------------------------------------------------------------------
	// Section 8 — Footer (y=384, h=51)
	// -----------------------------------------------------------------------
	footerY := sep2Y + secSepH
	footerRect := w32Rect{0, footerY, popW, popH}
	footerBrush := gdiBrush(clrFooterBg)
	w32FillRect(hdc, &footerRect, footerBrush)
	gdiDel(footerBrush)
}

// Suppress unused import warning
var _ = unsafe.Sizeof(0)
