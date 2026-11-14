//go:build windows

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/atomicfile"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/config"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/version"
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/webview2/edge"
)

const (
	windowClass = "LastfmScrobblerWindow"
	windowTitle = version.Name

	windowW, windowH       = 980, 720
	windowMinW, windowMinH = 860, 560
	miniW, miniH           = 480, 152
	miniMinW               = 288

	envWindowURL    = "LASTFM_SCROBBLER_WINDOW_URL"
	envWindowParent = "LASTFM_SCROBBLER_PARENT"
	envDevTools     = "LASTFM_SCROBBLER_DEVTOOLS"

	placeFile = "window-place.json"

	wmAppCheck  = wmApp + 1
	pageTimeout = 30 * time.Second
	wmAppQuit   = wmApp + 2
)

var themeColors = map[string]edge.COREWEBVIEW2_COLOR{
	"standard": {A: 255, R: 0xFB, G: 0xE3, B: 0xF4},
	"dark":     {A: 255, R: 0x1B, G: 0x14, B: 0x20},
	"brat":     {A: 255, R: 0x8A, G: 0xCE, B: 0x00},
	"glitch":   {A: 255, R: 0x07, G: 0x05, B: 0x0D},
	"fallen":   {A: 255, R: 0x07, G: 0x0B, B: 0x10},
}

func deskColor(theme string) edge.COREWEBVIEW2_COLOR {
	if c, ok := themeColors[theme]; ok {
		return c
	}
	return themeColors["standard"]
}

func newBrush(c edge.COREWEBVIEW2_COLOR) uintptr {
	brush, _, _ := procCreateSolidBrush.Call(uintptr(c.R) | uintptr(c.G)<<8 | uintptr(c.B)<<16)
	return brush
}

type appWindow struct {
	hwnd    uintptr
	web     *edge.Chromium
	log     *slog.Logger
	ready   bool
	loaded  bool
	failed  bool
	audio   bool
	showCmd int
	normal  rect
	max     bool
	active  bool
	place   string
	mini    bool
	pinned  bool
	miniAt  point
	hasMini bool
	miniW96 int32
	theme   string
}

var theWindow *appWindow

func cmdWindow(args []string) error {
	url := os.Getenv(envWindowURL)
	if url == "" && len(args) > 0 {
		url = args[0]
	}
	if !strings.HasPrefix(url, "http://127.0.0.1:") {
		return errors.New("не задан адрес окна")
	}
	log := slog.Default()

	if raiseAppWindow(0) {
		return nil
	}
	v, err := edge.InstalledVersion()
	switch {
	case err != nil:
		return fmt.Errorf("%w (%v)", errNoWebView, err)
	case v == "":
		return errNoWebView
	}
	loaderName := edge.LoaderName
	if edge.BuiltinLoader() {
		loaderName = "встроенный"
	}
	log.Info("найден WebView2", "version", v, "loader", loaderName)

	runtime.LockOSThread()
	if r, _, _ := procCoInitializeEx.Call(0, coinitApartmentThreaded); int32(r) < 0 {
		return fmt.Errorf("не удалось подготовить COM: 0x%08X", uint32(r))
	}
	setDPIAware()

	dir, err := config.StateDir()
	if err != nil {
		return err
	}
	w := &appWindow{log: log, place: filepath.Join(dir, placeFile), active: true}
	theWindow = w
	if err := w.create(); err != nil {
		return err
	}
	if err := w.embed(url, filepath.Join(dir, "webview")); err != nil {
		return err
	}
	w.watchParent()
	time.AfterFunc(pageTimeout, func() { postMessage(w.hwnd, wmAppCheck, 0, 0) })

	var m winMsg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			if w.failed {
				return errPageTimeout
			}
			return nil
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func (w *appWindow) create() error {
	var inst windows.Handle
	if err := windows.GetModuleHandleEx(0, nil, &inst); err != nil {
		return err
	}
	cls, _ := windows.UTF16PtrFromString(windowClass)
	title, _ := windows.UTF16PtrFromString(windowTitle)
	big, small := exeIcons()
	cursor, _, _ := procLoadCursorW.Call(0, idcArrow)
	r := w.initialRect()
	brush := newBrush(deskColor(w.theme))

	wc := wndClassEx{
		Size:       uint32(unsafe.Sizeof(wndClassEx{})),
		WndProc:    windows.NewCallback(wndProc),
		Instance:   inst,
		Icon:       big,
		Cursor:     windows.Handle(cursor),
		Background: windows.Handle(brush),
		ClassName:  cls,
		IconSm:     small,
	}
	if atom, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		return fmt.Errorf("регистрация класса окна: %w", err)
	}

	w.normal = r
	w.showCmd = swShowNormal
	exStyle := uintptr(wsExAppWindow)
	if w.mini {
		r = w.miniRect(monitorDPI(monitorOf(r, monitorToNearest)))
		if w.pinned {
			exStyle |= wsExTopmost
		}
	}
	hwnd, _, err := procCreateWindowExW.Call(
		exStyle,
		uintptr(unsafe.Pointer(cls)),
		uintptr(unsafe.Pointer(title)),
		wsPopup|wsClipChildren|wsSysMenu|wsMinimizeBox,
		uintptr(r.Left), uintptr(r.Top), uintptr(r.width()), uintptr(r.height()),
		0, 0, uintptr(inst), 0)
	if hwnd == 0 {
		return fmt.Errorf("создание окна: %w", err)
	}
	w.hwnd = hwnd

	pref := uint32(dwmCornerDoNotRound)
	procDwmSetWindowAttribute.Call(hwnd, dwmaWindowCornerPreference, uintptr(unsafe.Pointer(&pref)), 4)

	showWindow(hwnd, w.showCmd)
	procUpdateWindow.Call(hwnd)
	procSetForegroundWindow.Call(hwnd)
	return nil
}

func (w *appWindow) embed(url, dataDir string) error {
	os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", "--autoplay-policy=no-user-gesture-required")

	w.web = edge.NewChromium()
	w.web.DataPath = dataDir
	w.web.MessageCallback = w.onMessage
	w.web.NavigationCompletedCallback = func(*edge.ICoreWebView2, *edge.ICoreWebView2NavigationCompletedEventArgs) {
		w.log.Info("страница окна загружена")
	}
	if !w.web.Embed(w.hwnd) {
		if w.web.Err != nil {
			return fmt.Errorf("не удалось запустить WebView2: %w", w.web.Err)
		}
		return errors.New("не удалось запустить WebView2")
	}
	w.ready = true
	w.web.Resize()
	_ = w.web.Show()
	_ = w.web.NotifyParentWindowPositionChanged()
	w.web.Focus()

	debug := os.Getenv(envDevTools) != ""
	if s, err := w.web.GetSettings(); err == nil && s != nil {
		_ = s.PutAreDefaultContextMenusEnabled(debug)
		_ = s.PutAreDevToolsEnabled(debug)
		_ = s.PutIsStatusBarEnabled(false)
		_ = s.PutIsZoomControlEnabled(false)
		_ = s.PutIsPinchZoomEnabled(false)
		_ = s.PutAreBrowserAcceleratorKeysEnabled(debug)
	} else {
		w.log.Debug("не удалось прочитать настройки WebView2", "err", err)
	}
	w.paintBackground()
	w.web.Init(fmt.Sprintf("window.lfsStart={mini:%t,pinned:%t}", w.mini, w.pinned))
	w.web.Navigate(url)
	w.log.Info("окно открыто, WebView2 запущен")
	return nil
}

func (w *appWindow) paintBackground() {
	if c := w.web.GetController(); c != nil {
		if c2 := c.GetICoreWebView2Controller2(); c2 != nil {
			_ = c2.PutDefaultBackgroundColor(deskColor(w.theme))
		}
	}
}

func (w *appWindow) setTheme(theme string) {
	if _, ok := themeColors[theme]; !ok || theme == w.theme {
		return
	}
	w.theme = theme
	if brush := newBrush(deskColor(theme)); brush != 0 {
		if old, _, _ := procSetClassLongPtrW.Call(w.hwnd, gclpHbrBackground, brush); old != 0 {
			procDeleteObject.Call(old)
		}
	}
	w.paintBackground()
}

func (w *appWindow) watchParent() {
	pid, err := strconv.Atoi(os.Getenv(envWindowParent))
	if err != nil || pid <= 0 {
		return
	}
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		postMessage(w.hwnd, wmAppQuit, 0, 0)
		return
	}
	go func() {
		defer windows.CloseHandle(h)
		windows.WaitForSingleObject(h, windows.INFINITE)
		postMessage(w.hwnd, wmAppQuit, 0, 0)
	}()
}

func (w *appWindow) onMessage(m string) {
	cmd, arg, _ := strings.Cut(m, ":")
	switch cmd {
	case "ready":
		if !w.loaded {
			w.loaded = true
			w.log.Info("страница окна нарисована")
		}
		w.sendState()
	case "drag":
		w.startTracking(htCaption)
	case "resize":
		if ht, ok := resizeEdges[arg]; ok {
			w.startTracking(ht)
		}
	case "min":
		showWindow(w.hwnd, swMinimize)
	case "max", "mini":
		w.setMini(!w.mini)
	case "pin":
		w.pinned = !w.pinned
		if w.mini {
			procSetWindowPos.Call(w.hwnd, w.zOrder(), 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
		}
		w.sendState()
	case "theme":
		w.setTheme(arg)
	case "close":
		postMessage(w.hwnd, wmClose, 0, 0)
	case "audio":
		w.audio = arg == "on"
		if !w.audio && !isVisible(w.hwnd) {
			procDestroyWindow.Call(w.hwnd)
		}
	}
}

var resizeEdges = map[string]uintptr{
	"n": htTop, "s": htBottom, "e": htRight, "w": htLeft,
	"nw": htTopLeft, "ne": htTopRight, "sw": htBottomLeft, "se": htBottomRight,
}

func (w *appWindow) startTracking(ht uintptr) {
	if isZoomed(w.hwnd) || !mouseDown() {
		return
	}
	procReleaseCapture.Call()
	p := cursorPos()
	postMessage(w.hwnd, wmNCLButtonDown, ht, uintptr(uint16(p.X))|uintptr(uint16(p.Y))<<16)
}

func mouseDown() bool {
	vk := uintptr(vkLButton)
	if r, _, _ := procGetSystemMetrics.Call(smSwapButton); r != 0 {
		vk = vkRButton
	}
	r, _, _ := procGetAsyncKeyState.Call(vk)
	return r&0x8000 != 0
}

func (w *appWindow) sendState() {
	if w.ready {
		w.web.Eval(fmt.Sprintf("window.lfsFrame&&lfsFrame(%t,%t,%t,%t)", w.max, w.active, w.mini, w.pinned))
	}
}

func (w *appWindow) remember(hwnd uintptr) {
	if isZoomed(hwnd) || isIconic(hwnd) {
		return
	}
	r := windowRect(hwnd)
	if w.mini {
		w.miniAt, w.hasMini = point{r.Left, r.Top}, true
		if dpi := windowDPI(hwnd); dpi > 0 {
			w.miniW96 = int32(int64(r.width()) * 96 / int64(dpi))
		}
		return
	}
	w.normal = r
}

func (w *appWindow) zOrder() uintptr {
	if w.mini && w.pinned {
		return hwndTopmost
	}
	return hwndNotopmost
}

func (w *appWindow) setMini(on bool) {
	if on == w.mini {
		return
	}
	if isIconic(w.hwnd) || isZoomed(w.hwnd) {
		showWindow(w.hwnd, swRestore)
	}
	if on {
		w.normal = windowRect(w.hwnd)
	}
	w.mini = on
	r := w.normal
	if on {
		r = w.miniRect(windowDPI(w.hwnd))
	}
	procSetWindowPos.Call(w.hwnd, w.zOrder(), uintptr(r.Left), uintptr(r.Top), uintptr(r.width()), uintptr(r.height()), 0)
	w.sendState()
}

func (w *appWindow) miniRect(dpi uint32) rect {
	x, y := w.normal.Left, w.normal.Top
	if w.hasMini {
		x, y = w.miniAt.X, w.miniAt.Y
	}
	width := scaleDPI(miniW, dpi)
	if w.miniW96 > 0 {
		width = scaleDPI(w.miniW96, dpi)
	}
	width = max(scaleDPI(miniMinW, dpi), min(scaleDPI(miniW, dpi), width))
	r := rect{x, y, x + width, y + miniHeight(width)}
	return fitRect(r, monitorOf(r, monitorToNearest))
}

func miniHeight(width int32) int32 { return int32((int64(width)*miniH + miniW/2) / miniW) }

func (w *appWindow) keepAspect(edge uintptr, r *rect) {
	dpi := windowDPI(w.hwnd)
	width := r.width()
	if edge == wmszTop || edge == wmszBottom {
		width = int32(int64(r.height()) * miniW / miniH)
	}
	width = max(scaleDPI(miniMinW, dpi), min(scaleDPI(miniW, dpi), width))
	height := miniHeight(width)
	switch edge {
	case wmszLeft, wmszTopLeft, wmszBottomLeft:
		r.Left = r.Right - width
	default:
		r.Right = r.Left + width
	}
	switch edge {
	case wmszTop, wmszTopLeft, wmszTopRight:
		r.Top = r.Bottom - height
	default:
		r.Bottom = r.Top + height
	}
}

func wndProc(hwnd, msg, wp, lp uintptr) uintptr {
	w := theWindow
	if w == nil {
		r, _, _ := procDefWindowProcW.Call(hwnd, msg, wp, lp)
		return r
	}
	switch msg {
	case wmSize:
		if w.ready {
			w.web.Resize()
			_ = w.web.GetController().PutIsVisible(wp != sizeMinimized)
		}
		if wp == sizeMaximized || wp == sizeRestored {
			if maximized := wp == sizeMaximized; maximized != w.max {
				w.max = maximized
				w.sendState()
			}
		}
		w.remember(hwnd)
	case wmMove, wmMoving:
		if w.ready {
			_ = w.web.NotifyParentWindowPositionChanged()
		}
		if msg == wmMove {
			w.remember(hwnd)
		}
	case wmActivate:
		active := wp&0xFFFF != waInactive
		if active && w.ready {
			w.web.Focus()
		}
		if active != w.active {
			w.active = active
			w.sendState()
		}
	case wmGetMinMaxInfo:
		w.limits(hwnd, (*minMaxInfo)(lparamPtr(&lp)))
		return 0
	case wmSizing:
		if w.mini {
			w.keepAspect(wp, (*rect)(lparamPtr(&lp)))
			return 1
		}
	case wmDPIChanged:
		r := (*rect)(lparamPtr(&lp))
		procSetWindowPos.Call(hwnd, 0, uintptr(r.Left), uintptr(r.Top), uintptr(r.width()), uintptr(r.height()),
			swpNoZOrder|swpNoActivate)
		return 0
	case wmAppCheck:
		if !w.loaded {
			w.log.Error("страница не появилась во встроенном окне — открываю окно в Edge")
			w.failed = true
			procDestroyWindow.Call(hwnd)
		}
		return 0
	case wmClose:
		if w.audio {
			showWindow(hwnd, swHide)
			return 0
		}
		procDestroyWindow.Call(hwnd)
		return 0
	case wmAppQuit:
		procDestroyWindow.Call(hwnd)
		return 0
	case wmShowWindow:
		if wp != 0 && w.ready {
			_ = w.web.Show()
			_ = w.web.NotifyParentWindowPositionChanged()
		}
	case wmDestroy:
		w.save(hwnd)
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, msg, wp, lp)
	return r
}

func lparamPtr(lp *uintptr) unsafe.Pointer { return *(*unsafe.Pointer)(unsafe.Pointer(lp)) }

func (w *appWindow) limits(hwnd uintptr, mmi *minMaxInfo) {
	dpi := windowDPI(hwnd)
	if w.mini {
		small, big := scaleDPI(miniMinW, dpi), scaleDPI(miniW, dpi)
		mmi.MinTrackSize = point{small, miniHeight(small)}
		mmi.MaxTrackSize = point{big, miniHeight(big)}
		return
	}
	mmi.MinTrackSize = point{scaleDPI(windowMinW, dpi), scaleDPI(windowMinH, dpi)}
	mon, _, _ := procMonitorFromWindow.Call(hwnd, monitorToNearest)
	if full, work, ok := monitorAreas(mon); ok {
		mmi.MaxPosition = point{work.Left - full.Left, work.Top - full.Top}
		mmi.MaxSize = point{work.width(), work.height()}
	}
}

type windowPlace struct {
	X, Y, W, H int32
	DPI        uint32
	Mini       bool   `json:",omitempty"`
	MiniX      int32  `json:",omitempty"`
	MiniY      int32  `json:",omitempty"`
	MiniW      int32  `json:",omitempty"`
	HasMini    bool   `json:",omitempty"`
	Unpinned   bool   `json:",omitempty"`
	Theme      string `json:",omitempty"`
}

func (w *appWindow) initialRect() rect {
	w.pinned = true
	if data, err := os.ReadFile(w.place); err == nil {
		var p windowPlace
		if json.Unmarshal(data, &p) == nil && p.W > 0 && p.H > 0 {
			w.mini, w.pinned, w.theme = p.Mini, !p.Unpinned, p.Theme
			w.miniAt, w.hasMini, w.miniW96 = point{p.MiniX, p.MiniY}, p.HasMini, p.MiniW
			r := rect{p.X, p.Y, p.X + p.W, p.Y + p.H}
			if mon := monitorOf(r, monitorToNull); mon != 0 {
				if dpi := monitorDPI(mon); p.DPI > 0 && dpi != p.DPI {
					r.Right = r.Left + int32(int64(p.W)*int64(dpi)/int64(p.DPI))
					r.Bottom = r.Top + int32(int64(p.H)*int64(dpi)/int64(p.DPI))
				}
				return fitRect(r, mon)
			}
		}
	}
	c := cursorPos()
	mon := monitorOf(rect{c.X, c.Y, c.X + 1, c.Y + 1}, monitorToNearest)
	dpi := monitorDPI(mon)
	r := rect{0, 0, scaleDPI(windowW, dpi), scaleDPI(windowH, dpi)}
	if _, work, ok := monitorAreas(mon); ok {
		x := work.Left + (work.width()-r.width())/2
		y := work.Top + (work.height()-r.height())/2
		r = rect{x, y, x + r.width(), y + r.height()}
	}
	return fitRect(r, mon)
}

func fitRect(r rect, mon uintptr) rect {
	_, work, ok := monitorAreas(mon)
	if !ok {
		return r
	}
	wd, ht := min(r.width(), work.width()), min(r.height(), work.height())
	x := max(work.Left, min(r.Left, work.Right-wd))
	y := max(work.Top, min(r.Top, work.Bottom-ht))
	return rect{x, y, x + wd, y + ht}
}

func (w *appWindow) save(hwnd uintptr) {
	p := windowPlace{
		X: w.normal.Left, Y: w.normal.Top, W: w.normal.width(), H: w.normal.height(),
		DPI: windowDPI(hwnd), Mini: w.mini, MiniX: w.miniAt.X, MiniY: w.miniAt.Y, MiniW: w.miniW96,
		HasMini: w.hasMini, Unpinned: !w.pinned, Theme: w.theme,
	}
	if p.W <= 0 || p.H <= 0 {
		return
	}
	data, _ := json.Marshal(p)
	if err := atomicfile.Write(w.place, data); err != nil {
		w.log.Debug("не удалось сохранить место окна", "err", err)
	}
}

func exeIcons() (big, small windows.Handle) {
	exe, err := os.Executable()
	if err != nil {
		return 0, 0
	}
	p, _ := windows.UTF16PtrFromString(exe)
	procExtractIconExW.Call(uintptr(unsafe.Pointer(p)), 0,
		uintptr(unsafe.Pointer(&big)), uintptr(unsafe.Pointer(&small)), 1)
	return big, small
}
