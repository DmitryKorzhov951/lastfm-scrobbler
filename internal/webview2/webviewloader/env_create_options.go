//go:build windows

package webviewloader

import (
	"unicode/utf16"
	"unsafe"

	"github.com/DmitryKorzhov951/cpp_learn/lastfm-scrobbler/internal/webview2/combridge"
	"golang.org/x/sys/windows"
)

func WithBrowserExecutableFolder(folder string) option {
	return func(wvep *environmentOptions) {
		wvep.browserExecutableFolder = folder
	}
}

func WithUserDataFolder(folder string) option {
	return func(wvep *environmentOptions) {
		wvep.userDataFolder = folder
	}
}

func WithAdditionalBrowserArguments(args string) option {
	return func(wvep *environmentOptions) {
		wvep.additionalBrowserArguments = args
	}
}

func WithLanguage(lang string) option {
	return func(wvep *environmentOptions) {
		wvep.language = lang
	}
}

func WithTargetCompatibleBrowserVersion(version string) option {
	return func(wvep *environmentOptions) {
		wvep.targetCompatibleBrowserVersion = version
	}
}

func WithAllowSingleSignOnUsingOSPrimaryAccount(allow bool) option {
	return func(wvep *environmentOptions) {
		wvep.allowSingleSignOnUsingOSPrimaryAccount = allow
	}
}

func WithExclusiveUserDataFolderAccess(exclusive bool) option {
	return func(wvep *environmentOptions) {
		wvep.exclusiveUserDataFolderAccess = exclusive
	}
}

type option func(*environmentOptions)

var _ iCoreWebView2EnvironmentOptions = &environmentOptions{}
var _ iCoreWebView2EnvironmentOptions2 = &environmentOptions{}

type environmentOptions struct {
	browserExecutableFolder string
	userDataFolder          string
	preferCanary            bool

	additionalBrowserArguments             string
	language                               string
	targetCompatibleBrowserVersion         string
	allowSingleSignOnUsingOSPrimaryAccount bool
	exclusiveUserDataFolderAccess          bool
}

func (o *environmentOptions) AdditionalBrowserArguments() string {
	return o.additionalBrowserArguments
}

func (o *environmentOptions) Language() string {
	return o.language
}

func (o *environmentOptions) TargetCompatibleBrowserVersion() string {
	v := o.targetCompatibleBrowserVersion
	if v == "" {
		v = kMinimumCompatibleVersion
	}
	return v
}

func (o *environmentOptions) AllowSingleSignOnUsingOSPrimaryAccount() bool {
	return o.allowSingleSignOnUsingOSPrimaryAccount
}

func (o *environmentOptions) ExclusiveUserDataFolderAccess() bool {
	return o.exclusiveUserDataFolderAccess
}

type iCoreWebView2EnvironmentOptions interface {
	combridge.IUnknown

	AdditionalBrowserArguments() string
	Language() string
	TargetCompatibleBrowserVersion() string
	AllowSingleSignOnUsingOSPrimaryAccount() bool
}

type iCoreWebView2EnvironmentOptions2 interface {
	combridge.IUnknown

	ExclusiveUserDataFolderAccess() bool
}

func init() {
	combridge.RegisterVTable[combridge.IUnknown, iCoreWebView2EnvironmentOptions](
		"{2fde08a8-1e9a-4766-8c05-95a9ceb9d1c5}",
		_iCoreWebView2EnvironmentOptionsAdditionalBrowserArguments,
		_iCoreWebView2EnvironmentOptionsNOP,
		_iCoreWebView2EnvironmentOptionsLanguage,
		_iCoreWebView2EnvironmentOptionsNOP,
		_iCoreWebView2EnvironmentTargetCompatibleBrowserVersion,
		_iCoreWebView2EnvironmentOptionsNOP,
		_iCoreWebView2EnvironmentOptionsAllowSingleSignOnUsingOSPrimaryAccount,
		_iCoreWebView2EnvironmentOptionsNOP,
	)

	combridge.RegisterVTable[combridge.IUnknown, iCoreWebView2EnvironmentOptions2](
		"{ff85c98a-1ba7-4a6b-90c8-2b752c89e9e2}",
		_iCoreWebView2EnvironmentOptions2ExclusiveUserDataFolderAccess,
		_iCoreWebView2EnvironmentOptionsNOP,
	)
}
func _iCoreWebView2EnvironmentOptionsNOP(this uintptr) uintptr {
	return uintptr(windows.S_FALSE)
}

func _iCoreWebView2EnvironmentOptionsAdditionalBrowserArguments(this uintptr, value **uint16) uintptr {
	v := combridge.Resolve[iCoreWebView2EnvironmentOptions](this).AdditionalBrowserArguments()
	*value = stringToOleString(v)
	return uintptr(windows.S_OK)
}

func _iCoreWebView2EnvironmentOptionsLanguage(this uintptr, value **uint16) uintptr {
	args := combridge.Resolve[iCoreWebView2EnvironmentOptions](this).Language()
	*value = stringToOleString(args)
	return uintptr(windows.S_OK)
}

func _iCoreWebView2EnvironmentTargetCompatibleBrowserVersion(this uintptr, value **uint16) uintptr {
	args := combridge.Resolve[iCoreWebView2EnvironmentOptions](this).TargetCompatibleBrowserVersion()
	*value = stringToOleString(args)
	return uintptr(windows.S_OK)
}

func _iCoreWebView2EnvironmentOptionsAllowSingleSignOnUsingOSPrimaryAccount(this uintptr, value *int32) uintptr {
	v := combridge.Resolve[iCoreWebView2EnvironmentOptions](this).AllowSingleSignOnUsingOSPrimaryAccount()
	*value = boolToInt(v)
	return uintptr(windows.S_OK)
}

func _iCoreWebView2EnvironmentOptions2ExclusiveUserDataFolderAccess(this uintptr, value *int32) uintptr {
	v := combridge.Resolve[iCoreWebView2EnvironmentOptions2](this).ExclusiveUserDataFolderAccess()
	*value = boolToInt(v)
	return uintptr(windows.S_OK)
}

func stringToOleString(v string) *uint16 {
	wstr := utf16.Encode([]rune(v + "\x00"))
	lwstr := len(wstr)
	ptr := (*uint16)(coTaskMemAlloc(2 * lwstr))

	copy(unsafe.Slice(ptr, lwstr), wstr)

	return ptr
}

func boolToInt(v bool) int32 {
	if v {
		return 1
	}
	return 0
}
