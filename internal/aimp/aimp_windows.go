//go:build windows

package aimp

import (
	"errors"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/media"
)

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	procFindWindowW         = user32.NewProc("FindWindowW")
	procSendMessageTimeoutW = user32.NewProc("SendMessageTimeoutW")
	kernel32                = windows.NewLazySystemDLL("kernel32.dll")
	procOpenFileMappingW    = kernel32.NewProc("OpenFileMappingW")
)

const replyTimeout = 500

func Read(at time.Time) (st media.State, ok bool, err error) {
	name, err := windows.UTF16PtrFromString(RemoteName)
	if err != nil {
		return st, false, err
	}
	hwnd, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(name)))
	if hwnd == 0 {
		return st, false, nil
	}

	state, replied := send(hwnd, StatusPlayer)
	if !replied {
		return st, false, errors.New("AIMP не отвечает")
	}
	pos, _ := send(hwnd, StatusPos)

	buf, err := readShared(name)
	if err != nil {
		return st, false, err
	}
	info, err := Parse(buf)
	if err != nil {
		return st, false, err
	}
	return ToState(info, int(state), time.Duration(pos)*time.Second, at), true, nil
}

func send(hwnd, what uintptr) (uintptr, bool) {
	const smtoAbortIfHung = 0x0002
	var result uintptr
	r, _, _ := procSendMessageTimeoutW.Call(hwnd, WMCommand, StatusGet, what,
		smtoAbortIfHung, replyTimeout, uintptr(unsafe.Pointer(&result)))
	return result, r != 0
}

func readShared(name *uint16) ([]byte, error) {
	h, _, callErr := procOpenFileMappingW.Call(windows.FILE_MAP_READ, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return nil, callErr
	}
	defer windows.CloseHandle(windows.Handle(h))

	addr, err := windows.MapViewOfFile(windows.Handle(h), windows.FILE_MAP_READ, 0, 0, MapSize)
	if err != nil {
		return nil, err
	}
	defer windows.UnmapViewOfFile(addr)

	p := *(*unsafe.Pointer)(unsafe.Pointer(&addr))
	buf := make([]byte, MapSize)
	copy(buf, unsafe.Slice((*byte)(p), MapSize))
	return buf, nil
}
