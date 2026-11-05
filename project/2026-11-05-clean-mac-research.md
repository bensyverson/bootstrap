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
| Do the threads run smoothly? | Ben's wife's MacBook Air, with her OK, opening one HTML file | Nothing is installed |
| Does it work on a MacBook Neo? | A real Neo, if one turns up | Not required for shipping |

A new macOS account on the Mini can't test the first-run steps, because Homebrew and the Command Line Tools are installed for the whole machine. A VM can't judge graphics performance, which is why the Air has a job.
