//go:build windows

package aimp

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/cover"
)

const (
	cmdGetAlbumArt   = 10 + 19
	copyDataAlbumArt = 0x41495043
	wmCopyData       = 0x004A
	wmArtRequest     = 0x8000 + 0x31
	msgfltAllow      = 1
	artClass         = "LastfmScrobblerAIMPArt"
	artWait          = 1500 * time.Millisecond
	artReplyTimeout  = 1200
)

var hwndMessage = ^uintptr(2)

var (
	procRegisterClassExW            = user32.NewProc("RegisterClassExW")
	procCreateWindowExW             = user32.NewProc("CreateWindowExW")
	procDefWindowProcW              = user32.NewProc("DefWindowProcW")
	procGetMessageW                 = user32.NewProc("GetMessageW")
	procDispatchMessageW            = user32.NewProc("DispatchMessageW")
	procPostMessageW                = user32.NewProc("PostMessageW")
	procChangeWindowMessageFilterEx = user32.NewProc("ChangeWindowMessageFilterEx")
)

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
	X, Y    int32
	Private uint32
}

type copyData struct {
	Data uintptr
	Size uint32
	Ptr  uintptr
}

type artBox struct {
	once sync.Once
	hwnd uintptr
	err  error

	mu     sync.Mutex
	target uintptr
	seq    uint64
	reply  chan []byte
}

var box artBox

var (
	artTries = cover.NewAttempts()
	artMu    sync.Mutex
)

func trackArt(hwnd uintptr, info Info, at time.Time) []byte {
	artMu.Lock()
	defer artMu.Unlock()
	key := info.FileName + "\x00" + info.Artist + "\x00" + info.Title + "\x00" + info.Album
	if artTries.Due(key, at) {
		artTries.Put(key, AlbumArt(hwnd), at)
	}
	artTries.Sweep()
	return artTries.Get(key)
}

func AlbumArt(target uintptr) []byte {
	box.once.Do(box.start)
	if box.err != nil || target == 0 {
		return nil
	}
	ch := make(chan []byte, 1)
	box.mu.Lock()
	box.seq++
	seq := box.seq
	box.target, box.reply = target, ch
	box.mu.Unlock()

	if r, _, _ := procPostMessageW.Call(box.hwnd, wmArtRequest, 0, 0); r == 0 {
		box.finish(seq, nil)
	}
	select {
	case data := <-ch:
		return data
	case <-time.After(artWait):
		box.finish(seq, nil)
		return nil
	}
}

func (b *artBox) start() {
	ready := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		b.hwnd, b.err = createArtWindow()
		close(ready)
		if b.err != nil {
			return
		}
		var m winMsg
		for {
			r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(r) <= 0 {
				return
			}
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
	}()
	<-ready
}

func createArtWindow() (uintptr, error) {
	var inst windows.Handle
	if err := windows.GetModuleHandleEx(0, nil, &inst); err != nil {
		return 0, err
	}
	cls, _ := windows.UTF16PtrFromString(artClass)
	wc := wndClassEx{
		Size:      uint32(unsafe.Sizeof(wndClassEx{})),
		WndProc:   windows.NewCallback(artWndProc),
		Instance:  inst,
		ClassName: cls,
	}
	if r, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 && !errors.Is(err, windows.ERROR_CLASS_ALREADY_EXISTS) {
		return 0, fmt.Errorf("окно для обложек AIMP: %w", err)
	}
	hwnd, _, err := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), 0, 0,
		0, 0, 0, 0, hwndMessage, 0, uintptr(inst), 0)
	if hwnd == 0 {
		return 0, fmt.Errorf("окно для обложек AIMP: %w", err)
	}
	procChangeWindowMessageFilterEx.Call(hwnd, wmCopyData, msgfltAllow, 0)
	return hwnd, nil
}

func artWndProc(hwnd, msg, wp, lp uintptr) uintptr {
	switch msg {
	case wmArtRequest:
		box.ask(hwnd)
		return 0
	case wmCopyData:
		cd := (*copyData)(*(*unsafe.Pointer)(unsafe.Pointer(&lp)))
		if cd != nil && cd.Data == copyDataAlbumArt && cd.Size > 0 && cd.Size <= cover.MaxSize && cd.Ptr != 0 {
			src := unsafe.Slice((*byte)(*(*unsafe.Pointer)(unsafe.Pointer(&cd.Ptr))), cd.Size)
			box.deliver(append([]byte(nil), src...))
		}
		return 1
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, msg, wp, lp)
	return r
}

func (b *artBox) ask(self uintptr) {
	b.mu.Lock()
	target, seq, waiting := b.target, b.seq, b.reply != nil
	b.mu.Unlock()
	if !waiting {
		return
	}
	const smtoAbortIfHung = 0x0002
	var result uintptr
	r, _, _ := procSendMessageTimeoutW.Call(target, WMCommand, cmdGetAlbumArt, self,
		smtoAbortIfHung, artReplyTimeout, uintptr(unsafe.Pointer(&result)))
	if r == 0 || result == 0 {
		b.finish(seq, nil)
	}
}

func (b *artBox) deliver(data []byte) {
	b.mu.Lock()
	seq := b.seq
	b.mu.Unlock()
	b.finish(seq, data)
}

func (b *artBox) finish(seq uint64, data []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if seq != b.seq || b.reply == nil {
		return
	}
	b.reply <- data
	b.reply = nil
}
