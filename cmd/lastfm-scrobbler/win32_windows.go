//go:build windows

package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32 = windows.NewLazySystemDLL("user32.dll")

	procRegisterClassExW              = user32.NewProc("RegisterClassExW")
	procCreateWindowExW               = user32.NewProc("CreateWindowExW")
	procDefWindowProcW                = user32.NewProc("DefWindowProcW")
	procDestroyWindow                 = user32.NewProc("DestroyWindow")
	procShowWindow                    = user32.NewProc("ShowWindow")
	procUpdateWindow                  = user32.NewProc("UpdateWindow")
	procGetMessageW                   = user32.NewProc("GetMessageW")
	procTranslateMessage              = user32.NewProc("TranslateMessage")
	procDispatchMessageW              = user32.NewProc("DispatchMessageW")
	procPostQuitMessage               = user32.NewProc("PostQuitMessage")
	procPostMessageW                  = user32.NewProc("PostMessageW")
	procReleaseCapture                = user32.NewProc("ReleaseCapture")
	procSetWindowPos                  = user32.NewProc("SetWindowPos")
	procGetWindowRect                 = user32.NewProc("GetWindowRect")
	procIsZoomed                      = user32.NewProc("IsZoomed")
	procIsIconic                      = user32.NewProc("IsIconic")
	procIsWindowVisible               = user32.NewProc("IsWindowVisible")
	procSetForegroundWindow           = user32.NewProc("SetForegroundWindow")
	procAllowSetForegroundWindow      = user32.NewProc("AllowSetForegroundWindow")
	procFindWindowExW                 = user32.NewProc("FindWindowExW")
	procLoadCursorW                   = user32.NewProc("LoadCursorW")
	procGetCursorPos                  = user32.NewProc("GetCursorPos")
	procMonitorFromRect               = user32.NewProc("MonitorFromRect")
	procMonitorFromWindow             = user32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW               = user32.NewProc("GetMonitorInfoW")
	procGetDpiForWindow               = user32.NewProc("GetDpiForWindow")
	procGetAsyncKeyState              = user32.NewProc("GetAsyncKeyState")
	procGetSystemMetrics              = user32.NewProc("GetSystemMetrics")
	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procSetProcessDPIAware            = user32.NewProc("SetProcessDPIAware")

	procExtractIconExW        = windows.NewLazySystemDLL("shell32.dll").NewProc("ExtractIconExW")
	procCreateSolidBrush      = windows.NewLazySystemDLL("gdi32.dll").NewProc("CreateSolidBrush")
	procGetDpiForMonitor      = windows.NewLazySystemDLL("shcore.dll").NewProc("GetDpiForMonitor")
	procDwmSetWindowAttribute = windows.NewLazySystemDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute")
	procCoInitializeEx        = windows.NewLazySystemDLL("ole32.dll").NewProc("CoInitializeEx")
)

const (
	wsPopup        = 0x80000000
	wsClipChildren = 0x02000000
	wsSysMenu      = 0x00080000
	wsMinimizeBox  = 0x00020000
	wsMaximizeBox  = 0x00010000
	wsExAppWindow  = 0x00040000

	wmMove           = 0x0003
	wmSize           = 0x0005
	wmActivate       = 0x0006
	wmClose          = 0x0010
	wmShowWindow     = 0x0018
	wmGetMinMaxInfo  = 0x0024
	wmNCLButtonDown  = 0x00A1
	wmMoving         = 0x0216
	wmDPIChanged     = 0x02E0
	wmApp            = 0x8000
	wmDestroy        = 0x0002
	waInactive       = 0
	sizeRestored     = 0
	sizeMinimized    = 1
	sizeMaximized    = 2
	swHide           = 0
	swShowNormal     = 1
	swShowMaximized  = 3
	swShow           = 5
	swMinimize       = 6
	swRestore        = 9
	swpNoZOrder      = 0x0004
	swpNoActivate    = 0x0010
	monitorToNull    = 0
	monitorToNearest = 2
	idcArrow         = 32512
	smSwapButton     = 23
	vkLButton        = 0x01
	vkRButton        = 0x02
	asfwAny          = ^uintptr(0)

	htCaption     = 2
	htLeft        = 10
	htRight       = 11
	htTop         = 12
	htTopLeft     = 13
	htTopRight    = 14
	htBottom      = 15
	htBottomLeft  = 16
	htBottomRight = 17

	dpiPerMonitorV2 = ^uintptr(3)

	coinitApartmentThreaded = 2

	dwmaWindowCornerPreference = 33
	dwmCornerDoNotRound        = 1
)

type point struct{ X, Y int32 }

type rect struct{ Left, Top, Right, Bottom int32 }

func (r rect) width() int32  { return r.Right - r.Left }
func (r rect) height() int32 { return r.Bottom - r.Top }

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   windows.Handle
	Icon       windows.Handle
	Cursor     windows.Handle
	Background windows.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     windows.Handle
}

type winMsg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

type minMaxInfo struct {
	Reserved, MaxSize, MaxPosition, MinTrackSize, MaxTrackSize point
}

type monitorInfo struct {
	Size          uint32
	Monitor, Work rect
	Flags         uint32
}

func isZoomed(hwnd uintptr) bool  { r, _, _ := procIsZoomed.Call(hwnd); return r != 0 }
func isIconic(hwnd uintptr) bool  { r, _, _ := procIsIconic.Call(hwnd); return r != 0 }
func isVisible(hwnd uintptr) bool { r, _, _ := procIsWindowVisible.Call(hwnd); return r != 0 }

func showWindow(hwnd uintptr, cmd int) { procShowWindow.Call(hwnd, uintptr(cmd)) }

func postMessage(hwnd, msg, wp, lp uintptr) { procPostMessageW.Call(hwnd, msg, wp, lp) }

func windowRect(hwnd uintptr) rect {
	var r rect
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	return r
}

func cursorPos() point {
	var p point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	return p
}

func monitorOf(r rect, flags uintptr) uintptr {
	m, _, _ := procMonitorFromRect.Call(uintptr(unsafe.Pointer(&r)), flags)
	return m
}

func monitorAreas(mon uintptr) (full, work rect, ok bool) {
	mi := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
	r, _, _ := procGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&mi)))
	return mi.Monitor, mi.Work, r != 0
}

func monitorDPI(mon uintptr) uint32 {
	if procGetDpiForMonitor.Find() == nil {
		var x, y uint32
		if r, _, _ := procGetDpiForMonitor.Call(mon, 0, uintptr(unsafe.Pointer(&x)), uintptr(unsafe.Pointer(&y))); r == 0 && x > 0 {
			return x
		}
	}
	return 96
}

func windowDPI(hwnd uintptr) uint32 {
	if procGetDpiForWindow.Find() == nil {
		if r, _, _ := procGetDpiForWindow.Call(hwnd); r > 0 {
			return uint32(r)
		}
	}
	return 96
}

func scaleDPI(v int32, dpi uint32) int32 { return int32(int64(v) * int64(dpi) / 96) }

func setDPIAware() {
	if procSetProcessDpiAwarenessContext.Find() == nil {
		if r, _, _ := procSetProcessDpiAwarenessContext.Call(dpiPerMonitorV2); r != 0 {
			return
		}
	}
	procSetProcessDPIAware.Call()
}

func allowForeground() { procAllowSetForegroundWindow.Call(asfwAny) }

func findAppWindow(pid uint32) uintptr {
	cls, _ := windows.UTF16PtrFromString(windowClass)
	var hwnd uintptr
	for {
		hwnd, _, _ = procFindWindowExW.Call(0, hwnd, uintptr(unsafe.Pointer(cls)), 0)
		if hwnd == 0 {
			return 0
		}
		if pid == 0 {
			return hwnd
		}
		var owner uint32
		if _, err := windows.GetWindowThreadProcessId(windows.HWND(hwnd), &owner); err == nil && owner == pid {
			return hwnd
		}
	}
}

func raiseAppWindow(pid uint32) bool {
	hwnd := findAppWindow(pid)
	if hwnd == 0 {
		return false
	}
	switch {
	case !isVisible(hwnd):
		showWindow(hwnd, swShow)
	case isIconic(hwnd):
		showWindow(hwnd, swRestore)
	}
	procSetForegroundWindow.Call(hwnd)
	return true
}
