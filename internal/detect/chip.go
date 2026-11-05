package detect

import "syscall"

// Chip names this Mac's processor, or "" if macOS won't say.
func Chip() string {
	name, err := syscall.Sysctl("machdep.cpu.brand_string")
	if err != nil {
		return ""
	}
	return name
}
