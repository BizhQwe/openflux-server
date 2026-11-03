package terminal

import (
	"golang.org/x/sys/windows"
	"os"
)

func init() {
	_ = windows.SetConsoleOutputCP(65001)
	_ = windows.SetConsoleCP(65001)
}

func inputTerminal() bool {
	var mode uint32
	return windows.GetConsoleMode(windows.Handle(os.Stdin.Fd()), &mode) == nil
}

func outputColor() bool {
	var mode uint32
	handle := windows.Handle(os.Stdout.Fd())
	return windows.GetConsoleMode(handle, &mode) == nil && windows.SetConsoleMode(handle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING) == nil
}

func ConsoleSupportsCyrillic() bool {
	return true
}

func disableInputEcho() (func() error, error) {
	handle := windows.Handle(os.Stdin.Fd())
	var mode uint32
	if e := windows.GetConsoleMode(handle, &mode); e != nil {
		return nil, e
	}
	if e := windows.SetConsoleMode(handle, mode&^windows.ENABLE_ECHO_INPUT); e != nil {
		return nil, e
	}
	return func() error { return windows.SetConsoleMode(handle, mode) }, nil
}
