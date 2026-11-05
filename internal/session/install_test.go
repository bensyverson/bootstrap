package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bensyverson/bootstrap/internal/detect"
)

const cltPackage = "Library/Updates/047-91568/CLTools_Executables.pkg"

func noStore(t *testing.T) Store { return Store{Dir: filepath.Join(t.TempDir(), "none")} }

func fakeProbe(t *testing.T) detect.Probe {
	return detect.Probe{Root: t.TempDir(), Home: t.TempDir(), Running: func(string) bool { return false }}
}

func place(t *testing.T, root, rel string, mode os.FileMode) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, mode); err != nil {
		t.Fatal(err)
	}
}

// runningFor says an installer is running for its first n checks, then gone.
func runningFor(n int32) func(string) bool {
	var calls atomic.Int32
	return func(string) bool { return calls.Add(1) <= n }
}

func TestWaitInstallSeesAStartDuringTheWait(t *testing.T) {
	probe := fakeProbe(t)
	go func() {
		time.Sleep(150 * time.Millisecond)
		place(t, probe.Root, cltPackage, 0o644)
	}()
	got, err := noStore(t).WaitInstall(context.Background(), probe, "clt", 5*time.Second)
	if err != nil || got.Event != Started {
		t.Errorf("got %+v, err %v", got, err)
	}
}

func TestWaitInstallIgnoresAStartFromBefore(t *testing.T) {
	probe := fakeProbe(t)
	place(t, probe.Root, cltPackage, 0o644)
	if got, err := noStore(t).WaitInstall(context.Background(), probe, "clt", 200*time.Millisecond); !errors.Is(err, ErrWaitTimeout) {
		t.Errorf("a folder from an earlier try, or from the last wait, isn't a new start: %+v, %v", got, err)
	}
}

func TestWaitInstallIgnoresFoldersWithoutAppleTools(t *testing.T) {
	probe := fakeProbe(t)
	go func() {
		time.Sleep(100 * time.Millisecond)
		place(t, probe.Root, "Library/Updates/index.plist", 0o644)
		place(t, probe.Root, "Library/Updates/012-34567/Safari.pkg", 0o644)
	}()
	if got, err := noStore(t).WaitInstall(context.Background(), probe, "clt", 400*time.Millisecond); !errors.Is(err, ErrWaitTimeout) {
		t.Errorf("Apple's disk check writes index.plist with no download: %+v, %v", got, err)
	}
}

func TestWaitInstallSeesACancel(t *testing.T) {
	for _, id := range []string{"clt", "homebrew"} {
		probe := fakeProbe(t)
		probe.Running = runningFor(3)
		got, err := noStore(t).WaitInstall(context.Background(), probe, id, 5*time.Second)
		if err != nil || got.Event != Canceled {
			t.Errorf("%s: got %+v, err %v", id, got, err)
		}
	}
}

func TestWaitInstallNeedsToSeeTheInstallerBeforeCallingItCanceled(t *testing.T) {
	probe := fakeProbe(t)
	if got, err := noStore(t).WaitInstall(context.Background(), probe, "clt", 200*time.Millisecond); !errors.Is(err, ErrWaitTimeout) {
		t.Errorf("an installer that hasn't started yet hasn't been canceled: %+v, %v", got, err)
	}
}

func TestWaitInstallPrefersArrivalToACancel(t *testing.T) {
	probe := fakeProbe(t)
	probe.Running = runningFor(1)
	place(t, probe.Root, "Library/Developer/CommandLineTools/usr/bin/git", 0o755)
	got, err := noStore(t).WaitInstall(context.Background(), probe, "clt", 5*time.Second)
	if err != nil || got.Event != Arrived || got.Path != "/Library/Developer/CommandLineTools/usr/bin/git" {
		t.Errorf("the installer quits once it's done: %+v, %v", got, err)
	}
}

func TestWaitInstallHasNoStartForHomebrew(t *testing.T) {
	probe := fakeProbe(t)
	go func() {
		time.Sleep(100 * time.Millisecond)
		place(t, probe.Root, cltPackage, 0o644)
	}()
	if got, err := noStore(t).WaitInstall(context.Background(), probe, "homebrew", 400*time.Millisecond); !errors.Is(err, ErrWaitTimeout) {
		t.Errorf("got %+v, %v", got, err)
	}
}

// Apple can download two versions of its tools, each into its own folder, a moment
// apart. In the clt3 VM run the second folder appeared after an agent's next wait
// began, and that wait returned started again instead of waiting for Git.
func TestWaitInstallIgnoresASecondFolderOnceUnderWay(t *testing.T) {
	probe := fakeProbe(t)
	probe.Running = func(string) bool { return true }
	place(t, probe.Root, cltPackage, 0o644)
	go func() {
		time.Sleep(100 * time.Millisecond)
		place(t, probe.Root, "Library/Updates/140-17812/CLTools_Executables.pkg", 0o644)
	}()
	if got, err := noStore(t).WaitInstall(context.Background(), probe, "clt", 400*time.Millisecond); !errors.Is(err, ErrWaitTimeout) {
		t.Errorf("a second folder during an install under way isn't a new start: %+v, %v", got, err)
	}
}

// A folder left by an earlier try, with no installer running, doesn't hide a real start.
func TestWaitInstallSeesAStartBesideALeftoverFolder(t *testing.T) {
	probe := fakeProbe(t)
	place(t, probe.Root, cltPackage, 0o644)
	go func() {
		time.Sleep(150 * time.Millisecond)
		place(t, probe.Root, "Library/Updates/140-17812/CLTools_Executables.pkg", 0o644)
	}()
	got, err := noStore(t).WaitInstall(context.Background(), probe, "clt", 5*time.Second)
	if err != nil || got.Event != Started {
		t.Errorf("got %+v, err %v", got, err)
	}
}

// The flow watches from inside the server, where a beat would claim an agent that isn't there.
func TestWatchInstallLeavesNoBeat(t *testing.T) {
	store := begun(t)
	before := load(t, store).Seq
	probe := fakeProbe(t)
	place(t, probe.Root, "Library/Developer/CommandLineTools/usr/bin/git", 0o755)
	got, err := WatchInstall(context.Background(), probe, "clt", 5*time.Second)
	if err != nil || got.Event != Arrived {
		t.Errorf("got %+v, err %v", got, err)
	}
	if after := load(t, store).Seq; after != before {
		t.Errorf("the log moved from %d to %d", before, after)
	}
}
