//go:build windows
// +build windows

package edge

import (
	"unsafe"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/webview2/w32"
)

func (e *Chromium) Resize() {
	if e.controller == nil {
		return
	}

	var bounds w32.Rect
	w32.User32GetClientRect.Call(e.hwnd, uintptr(unsafe.Pointer(&bounds)))

	words := (*[2]uintptr)(unsafe.Pointer(&bounds))
	e.controller.vtbl.PutBounds.Call(
		uintptr(unsafe.Pointer(e.controller)),
		words[0],
		words[1],
	)
}
