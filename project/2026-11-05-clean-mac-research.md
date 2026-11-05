# What we can learn without a clean Mac

Ben can't wipe or buy a Mac for testing. This records what research and local checks settled on 2026-11-05, what they can't settle, and where the rest can be tested.

## Settled

**Homebrew skips Apple's tools when one specific file exists.** `install.sh` line 452 installs the Command Line Tools only when `/Library/Developer/CommandLineTools/usr/bin/git` is missing. So running `xcode-select --install` first really does make it a separate step. Read from [`install.sh`](https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh), fetched with `curl` on 2026-11-05.

**That file, not `xcode-select -p`, is the right check.** On a Mac with only Xcode, `xcode-select -p` succeeds, yet Homebrew still installs the Command Line Tools because that file is missing. This Mac's `xcode-select -p` prints the Xcode path.

**Homebrew can install those tools itself, with no Apple window.** Lines 885 to 906 use `softwareupdate` with `sudo`, and fall back to `xcode-select --install` only if that fails and a terminal is attached (line 912). That makes a separate Apple step optional, not required.

**`curl` sets no quarantine flag on this Mac.** `xattr -l` printed nothing for a file fetched with `curl -sL` from GitHub on macOS 27.0.1. Community reports agree that `curl` downloads skip the Gatekeeper check that browser downloads get ([Apple forums](https://developer.apple.com/forums/thread/773755), [rclone forum](https://forum.rclone.org/t/1-67-binary-for-macos-not-notarized/47276)). None of those is Apple documentation, and Apple could tighten this, so the binary is still signed and notarized.

**A clean macOS VM is available without wiping anything.** Tart's `macos-*-vanilla` images are described as a macOS install with tweaks like auto-login and no additional software, while the `-base` images add Homebrew ([macos-image-templates](https://github.com/Monoxer/macos-image-templates)). A maintainer says that description is [somewhat outdated](https://github.com/cirruslabs/macos-image-templates/discussions/288), so check `/Library/Developer/CommandLineTools` in the VM first.

## Not settled by research

- Whether an agent will download and run the signed binary from a pasted link. This is a question about agent behavior, so it needs a real trial.
- How long the Command Line Tools take to install. I found no reliable figure, so the page says "This can take a while" and shows no estimate.
- Whether `xcode-select --install` can be detected reliably as it finishes. Polling for the file above should work, but it hasn't been run.
- How smoothly the threads run on a MacBook Neo or other low-powered Mac.

## Where each question gets tested

| Question | Where | What it changes on that machine |
|---|---|---|
| Does an agent run the binary from a pasted link? | Mac Mini, new folder, fresh session | Almost nothing |
| Does the full flow work from nothing? | Clean macOS VM on the Mac Mini | Nothing outside the VM |
| Do the threads run smoothly? | The Mac Mini's own display, opening the page | Nothing is installed |
| Does it work on a MacBook Neo? | A real Neo, if one turns up | Not required for shipping |

A new macOS account on the Mini can't test the first-run steps, because Homebrew and the Command Line Tools are installed for the whole machine. A VM can't judge graphics performance, which is why that check runs on the Mini itself.

## The Mac Mini, as found

Read over SSH as `agent@sally.local` on 2026-11-05, using `sw_vers`, `sysctl`, `df -h`, `du -sh` and `system_profiler SPDisplaysDataType`.

- **Machine:** Macmini9,1 (Apple M1) with 8GB of memory, on macOS 26.6.2. There are two accounts, `ben@` and `agent@`.
- **Disk:** 45GB free at first, with 33GB of that account's caches in `go-build`. After `go clean -cache` and removing `.build` folders, 78GB was free (`df -h`). The `macos-tahoe-vanilla` image is 24GB compressed and 50GB uncompressed, from its registry manifest, so it fits.
- **Display:** `system_profiler` reports none, but only because it was off. Ben says the Mini has one, so an M1 with 8GB can judge the threads natively, and the Air isn't needed for that.
- **Already there:** the Command Line Tools, Homebrew and, for `agent@`, Claude Code 2.1.260. So the agent trial works on this account, but only the VM can test the first-run steps.
- **Limits on `agent@`:** `/opt/homebrew` belongs to `ben`, so `agent@` can't `brew install`, and it has no passwordless `sudo`.
- **Tart:** 2.41.1 runs from `~agent/tart-install`, unpacked from its GitHub release rather than installed with Homebrew, so `ben@` is unchanged. The project now lives at `openai/tart`, under the Functional Source License.
- **Not there:** OpenCode.

**Tart is the efficient choice.** Apple's `container` runs Linux, so it can't test macOS. Every macOS VM tool on Apple silicon sits on the same Virtualization framework, so there's no faster engine to pick. Tart adds no always-running app.
