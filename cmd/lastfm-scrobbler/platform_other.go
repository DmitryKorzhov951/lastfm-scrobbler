//go:build !windows

package main

import (
	"context"
	"errors"
)

var errOnlyWindows = errors.New("программа работает только в Windows")

const autostartSupported = false

func setupBackground() error                    { return errOnlyWindows }
func acquireInstance() (func(), error)          { return func() {}, nil }
func watchStop(context.Context) <-chan struct{} { return nil }
func watchShow(context.Context) <-chan struct{} { return nil }
func replaceRunning() func()                    { return nil }
func refreshAutostart(*env)                     {}
func windowRunning() bool                       { return false }
func showRunningWindow(string) error            { return errOnlyWindows }
func openWindow(string) error                   { return errOnlyWindows }
func openURL(string) error                      { return errOnlyWindows }
func openFolder(string) error                   { return errOnlyWindows }
func logDir() string                            { return "" }
func autostartEnabled() bool                    { return false }
func setAutostart(*env, bool) error             { return errOnlyWindows }
func cmdWindow([]string) error                  { return errOnlyWindows }
