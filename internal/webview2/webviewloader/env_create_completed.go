//go:build windows

package webviewloader

import (
	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/webview2/combridge"
)

type HRESULT int32

type ICoreWebView2Environment = combridge.IUnknownImpl

type ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandler interface {
	EnvironmentCompleted(errorCode HRESULT, createdEnvironment *ICoreWebView2Environment) HRESULT
}

type iCoreWebView2CreateCoreWebView2EnvironmentCompletedHandler interface {
	combridge.IUnknown
	ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandler
}

func init() {
	combridge.RegisterVTable[combridge.IUnknown, iCoreWebView2CreateCoreWebView2EnvironmentCompletedHandler](
		"{4e8a3389-c9d8-4bd2-b6b5-124fee6cc14d}",
		_iCoreWebView2CreateCoreWebView2EnvironmentCompletedHandlerInvoke,
	)
}

func _iCoreWebView2CreateCoreWebView2EnvironmentCompletedHandlerInvoke(this uintptr, errorCode HRESULT, env *combridge.IUnknownImpl) uintptr {
	res := combridge.Resolve[iCoreWebView2CreateCoreWebView2EnvironmentCompletedHandler](this).EnvironmentCompleted(errorCode, env)
	return uintptr(res)
}
