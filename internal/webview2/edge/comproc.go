//go:build windows

package edge

import (
	"golang.org/x/sys/windows"
)

type ComProc uintptr

func NewComProc(fn interface{}) ComProc {
	return ComProc(windows.NewCallback(fn))
}
