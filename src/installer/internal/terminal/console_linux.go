package terminal

import (
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

func inputTerminal() bool {
	_, e := unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TCGETS)
	return e == nil
}

func outputColor() bool {
	_, e := unix.IoctlGetTermios(int(os.Stdout.Fd()), unix.TCGETS)
	return e == nil && os.Getenv("TERM") != "dumb"
}

func ConsoleSupportsCyrillic() bool {
	if os.Getenv("OPENFLUX_FORCE_CYRILLIC") != "" {
		return true
	}
	if os.Getenv("TERM") == "linux" {
		return false
	}
	for _, fd := range []string{"/proc/self/fd/0", "/proc/self/fd/1"} {
		if target, err := os.Readlink(fd); err == nil {
			if strings.HasPrefix(target, "/dev/tty") && !strings.HasPrefix(target, "/dev/ttyS") && !strings.HasPrefix(target, "/dev/pts/") {
				return false
			}
		}
	}
	return true
}

func disableInputEcho() (func() error, error) {
	fd := int(os.Stdin.Fd())
	current, e := unix.IoctlGetTermios(fd, unix.TCGETS)
	if e != nil {
		return nil, e
	}
	previous := *current
	current.Lflag &^= unix.ECHO
	if e = unix.IoctlSetTermios(fd, unix.TCSETS, current); e != nil {
		return nil, e
	}
	return func() error { return unix.IoctlSetTermios(fd, unix.TCSETS, &previous) }, nil
}
