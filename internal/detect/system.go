package detect

import (
	"os/exec"
	"syscall"
)

// AppleStepNeeds is the free space Apple's tools wait for, rounded up from the
// 19181109136 bytes softwareupdated logged on macOS 26.6.2
// (project/2026-11-05-vm-run-findings.md). The install itself takes about 1.9 GB.
const AppleStepNeeds uint64 = 20_000_000_000

// Space is whether there's room for Apple's step.
type Space string

const (
	SpaceEnough  Space = "enough"
	SpaceShort   Space = "short"
	SpaceUnknown Space = "unknown"
)

// Disk is the free space on the startup disk.
type Disk struct {
	Free      uint64 `json:"free_bytes"`
	AppleStep Space  `json:"apple_step"`
}

// Running reports whether a process's command line contains pattern.
func Running(pattern string) bool {
	return exec.Command("/usr/bin/pgrep", "-f", pattern).Run() == nil
}

// FreeBytes is the space available to this user on the disk holding path.
func FreeBytes(path string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	return st.Bavail * uint64(st.Bsize), nil
}

func inspectDisk(p Probe) Disk {
	if p.Free == nil {
		return Disk{AppleStep: SpaceUnknown}
	}
	free, err := p.Free(resolve(p, "/"))
	if err != nil {
		return Disk{AppleStep: SpaceUnknown}
	}
	disk := Disk{Free: free, AppleStep: SpaceShort}
	if free >= AppleStepNeeds {
		disk.AppleStep = SpaceEnough
	}
	return disk
}
