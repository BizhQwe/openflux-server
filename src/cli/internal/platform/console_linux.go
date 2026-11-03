package platform

import (
	"golang.org/x/sys/unix"
	"os"
)

func ConsoleColor() bool {
	_, e := unix.IoctlGetTermios(int(os.Stdout.Fd()), unix.TCGETS)
	return e == nil && os.Getenv("TERM") != "dumb"
}

// ConsoleSupportsCyrillic checks whether the active console supports Cyrillic UTF-8 rendering.
// On the Linux kernel virtual console (TERM=linux, /dev/tty0..63) on minimal/router firmware
// (such as OpenWrt, Keenetic, Padavan), only ASCII/CP437 glyphs exist in the kernel font,
// which causes Cyrillic UTF-8 characters to render as solid blocks (■).
func ConsoleSupportsCyrillic() bool {
	if os.Getenv("OPENFLUX_FORCE_CYRILLIC") != "" {
		return true
	}
	term := os.Getenv("TERM")
	if term == "linux" {
		return false
	}
	for _, fd := range []string{"/proc/self/fd/0", "/proc/self/fd/1"} {
		if target, err := os.Readlink(fd); err == nil {
			if strings.HasPrefix(target, "/dev/tty") && !strings.HasPrefix(target, "/dev/ttyS") && !strings.HasPrefix(target, "/dev/pts/") {
				return false
			}
		}
	}
	for _, env := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if val := os.Getenv(env); val != "" {
			if val == "C" || val == "POSIX" {
				return false
			}
			break
		}
	}
	return true
}
