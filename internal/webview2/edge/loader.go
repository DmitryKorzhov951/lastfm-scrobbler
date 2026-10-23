//go:build windows

package edge

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const LoaderName = "WebView2Loader.dll"

var (
	loaderOnce sync.Once
	loaderDLL  *windows.DLL
	loaderErr  error
)

func loader() (*windows.DLL, error) {
	loaderOnce.Do(func() {
		exe, err := os.Executable()
		if err != nil {
			loaderErr = err
			return
		}
		path := filepath.Join(filepath.Dir(exe), LoaderName)
		if loaderDLL, err = windows.LoadDLL(path); err != nil {
			loaderErr = fmt.Errorf("не удалось загрузить %s: %w", path, err)
		}
	})
	return loaderDLL, loaderErr
}

func InstalledVersion() (string, error) {
	dll, err := loader()
	if err != nil {
		return "", err
	}
	proc, err := dll.FindProc("GetAvailableCoreWebView2BrowserVersionString")
	if err != nil {
		return "", err
	}
	var result *uint16
	hr, _, _ := proc.Call(0, uintptr(unsafe.Pointer(&result)))
	defer windows.CoTaskMemFree(unsafe.Pointer(result))
	if uint32(hr) != 0 {
		if uint32(hr)&0xFFFF == uint32(windows.ERROR_FILE_NOT_FOUND) {
			return "", nil
		}
		return "", fmt.Errorf("GetAvailableCoreWebView2BrowserVersionString: 0x%08X", uint32(hr))
	}
	return windows.UTF16PtrToString(result), nil
}

func createCoreWebView2EnvironmentWithOptions(browserExecutableFolder, userDataFolder *uint16, environmentOptions uintptr, handler *iCoreWebView2CreateCoreWebView2EnvironmentCompletedHandler) (uintptr, error) {
	dll, err := loader()
	if err != nil {
		return 0, err
	}
	proc, err := dll.FindProc("CreateCoreWebView2EnvironmentWithOptions")
	if err != nil {
		return 0, err
	}
	res, _, _ := proc.Call(
		uintptr(unsafe.Pointer(browserExecutableFolder)),
		uintptr(unsafe.Pointer(userDataFolder)),
		environmentOptions,
		uintptr(unsafe.Pointer(handler)),
	)
	return res, nil
}
