package terminal

import (
	"golang.org/x/sys/unix"
	"os"
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
	term := os.Getenv("TERM")
	if term == "linux" || term == "dumb" || term == "" {
		return false
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
