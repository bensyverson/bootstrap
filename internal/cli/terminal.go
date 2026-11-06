package cli

import (
	"os"
	"syscall"
	"unsafe"
)

// isTerminal reports whether f is a terminal, by asking for its settings as the
// standard library's own terminal code does. A pipe or /dev/null has none.
func isTerminal(f *os.File) bool {
	var settings syscall.Termios
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), syscall.TIOCGETA, uintptr(unsafe.Pointer(&settings)))
	return errno == 0
}
