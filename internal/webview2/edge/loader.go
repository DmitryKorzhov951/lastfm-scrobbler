//go:build windows

package edge

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"unsafe"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/webview2/webviewloader"
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

func BuiltinLoader() bool {
	_, err := loader()
	return err != nil
}

func InstalledVersion() (string, error) {
	dll, err := loader()
	if err != nil {
		return webviewloader.GetAvailableCoreWebView2BrowserVersionString("")
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

type builtinEnvHandler struct {
	h *iCoreWebView2CreateCoreWebView2EnvironmentCompletedHandler
}

func (b builtinEnvHandler) EnvironmentCompleted(code webviewloader.HRESULT, env *webviewloader.ICoreWebView2Environment) webviewloader.HRESULT {
	res := b.h.impl.EnvironmentCompleted(uintptr(code), (*ICoreWebView2Environment)(unsafe.Pointer(env)))
	return webviewloader.HRESULT(int32(res))
}

func createCoreWebView2EnvironmentWithOptions(browserExecutableFolder, userDataFolder *uint16, environmentOptions uintptr, handler *iCoreWebView2CreateCoreWebView2EnvironmentCompletedHandler) (uintptr, error) {
	dll, err := loader()
	if err != nil {
		args := os.Getenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS")
		return 0, webviewloader.CreateCoreWebView2EnvironmentWithOptions(builtinEnvHandler{handler},
			webviewloader.WithUserDataFolder(windows.UTF16PtrToString(userDataFolder)),
			webviewloader.WithAdditionalBrowserArguments(args))
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
