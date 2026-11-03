package platform

import (
	"golang.org/x/sys/windows"
	"os"
)

func init() {
	// Ensure standard Windows consoles (cmd.exe/PowerShell) use UTF-8 output code page
	_ = windows.SetConsoleOutputCP(65001)
	_ = windows.SetConsoleCP(65001)
}

func ConsoleColor() bool {
	var mode uint32
	handle := windows.Handle(os.Stdout.Fd())
	return windows.GetConsoleMode(handle, &mode) == nil && windows.SetConsoleMode(handle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING) == nil
}

func ConsoleSupportsCyrillic() bool {
	return true
}
