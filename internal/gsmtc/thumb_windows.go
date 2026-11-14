//go:build windows

package gsmtc

import (
	"errors"
	"fmt"
	"syscall"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/saltosystems/winrt-go/windows/foundation"
	"github.com/saltosystems/winrt-go/windows/media/control"
	"golang.org/x/sys/windows"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/cover"
)

var (
	modShcore                              = windows.NewLazySystemDLL("shcore.dll")
	procCreateStreamOverRandomAccessStream = modShcore.NewProc("CreateStreamOverRandomAccessStream")
	iidIStream                             = ole.NewGUID("0000000C-0000-0000-C000-000000000046")
)

const thumbTimeout = 3 * time.Second

func readThumbnail(mp *control.GlobalSystemMediaTransportControlsSessionMediaProperties) ([]byte, error) {
	ref, err := mp.GetThumbnail()
	if err != nil || ref == nil {
		return nil, err
	}
	defer ref.Release()

	op, err := ref.OpenReadAsync()
	if err != nil {
		return nil, err
	}
	defer op.Release()
	ptr, err := awaitPolling(op, thumbTimeout)
	if err != nil || ptr == nil {
		return nil, err
	}
	stream := (*ole.IUnknown)(ptr)
	defer stream.Release()

	if err := procCreateStreamOverRandomAccessStream.Find(); err != nil {
		return nil, err
	}
	var is *ole.IUnknown
	hr, _, _ := procCreateStreamOverRandomAccessStream.Call(
		uintptr(unsafe.Pointer(stream)),
		uintptr(unsafe.Pointer(iidIStream)),
		uintptr(unsafe.Pointer(&is)))
	if int32(hr) < 0 || is == nil {
		return nil, fmt.Errorf("CreateStreamOverRandomAccessStream: 0x%08X", uint32(hr))
	}
	defer is.Release()
	return readStream(is, cover.MaxSize)
}

func readStream(s *ole.IUnknown, limit int) ([]byte, error) {
	read := (*[4]uintptr)(unsafe.Pointer(s.RawVTable))[3]
	buf := make([]byte, 64<<10)
	var out []byte
	for {
		var n uint32
		hr, _, _ := syscall.SyscallN(read,
			uintptr(unsafe.Pointer(s)),
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(len(buf)),
			uintptr(unsafe.Pointer(&n)))
		if int32(hr) < 0 {
			return nil, fmt.Errorf("чтение обложки: 0x%08X", uint32(hr))
		}
		if n > uint32(len(buf)) {
			n = uint32(len(buf))
		}
		out = append(out, buf[:n]...)
		if len(out) > limit {
			return nil, errors.New("обложка слишком большая")
		}
		if n == 0 || hr == 1 {
			return out, nil
		}
	}
}

func awaitPolling(op *foundation.IAsyncOperation, limit time.Duration) (unsafe.Pointer, error) {
	itf, err := op.QueryInterface(ole.NewGUID(foundation.GUIDIAsyncInfo))
	if err != nil {
		return nil, err
	}
	info := (*foundation.IAsyncInfo)(unsafe.Pointer(itf))
	defer info.Release()

	deadline := time.Now().Add(limit)
	for {
		st, err := info.GetStatus()
		if err != nil {
			return nil, err
		}
		switch st {
		case foundation.AsyncStatusCompleted:
			return op.GetResults()
		case foundation.AsyncStatusStarted:
			if time.Now().After(deadline) {
				_ = info.Cancel()
				return nil, errors.New("Windows не отдала обложку вовремя")
			}
			time.Sleep(15 * time.Millisecond)
		default:
			return nil, fmt.Errorf("чтение обложки завершилось со статусом %d", st)
		}
	}
}
