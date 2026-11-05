# First VM run: Homebrew's paste command is blocked, and its `.pkg` works (2026-11-05)

**The headline:** on macOS 26.6.2, Apple blocks a pasted Homebrew command with a malware warning, so the `terminal` screen can't stay as designed. Homebrew's signed `.pkg` installed with no Terminal and no "Next steps". This run didn't test everything, and "Still open" below lists what a fresh clone has to answer.

**The setup:** Tart VM `clean`, cloned from `ghcr.io/cirruslabs/macos-tahoe-vanilla`, on the Mac Mini. macOS 26.6.2, arm64. Ben clicked over Screen Sharing and commands ran over ssh from the Mini with `~agent/tart-install/vmrun.exp`.

**Before anything ran:** `xcode-select -p` exited 2, `/Library/Developer/CommandLineTools` didn't exist, `pkgutil --pkgs | grep -iE "CLTools|Xcode"` found nothing, and there was no Xcode or Homebrew.

## How this VM differs from a real new Mac

- **Gatekeeper is off.** `spctl -a -vv -t install` printed `override=security disabled`. This run can't show that Gatekeeper lets anything through.
- **Passwordless `sudo` came with the image.** `/etc/sudoers.d/admin-nopasswd` allowed it. I removed that file before the Homebrew attempts, and `sudo -n true` then said a password was required.
- **The account's display name is "Managed via Tart".** The installer filled that in as the username, and Ben's first login there failed. It went through after I suggested typing `admin`, so a real Mac's dialog isn't shown to have this problem. `dscl . -authonly admin admin` confirmed the password.
- **It runs macOS 26, not the macOS 15 floor.**
- **Apple's step was started over ssh** (`launchctl asuser 501 xcode-select --install`), not by an agent in the VM's own Terminal.

## What happened

| Question | What happened | How |
|---|---|---|
| Does Apple's window come up in front? | No. It opened behind Terminal both times, seen by Ben. | Two starts of `xcode-select --install` |
| Can the agent bring it forward? | Yes, once. Terminal was frontmost, and after `open -a` the installer was, with the same pid, no prompt, and Ben saw it come forward. | `open -a "/System/Library/CoreServices/Install Command Line Developer Tools.app"`, `lsappinfo front` |
| What signals the end of Apple's install? | The git file, the receipt, `xcode-select -p` and git running all flipped in the same poll, at 18:00:35. The installer exited at 18:00:45, presumably when Ben clicked Done. | A 1-second poll, `/tmp/clt-signals.log` |
| Is there a signal that they clicked Install? | None among those logged: nothing changed from the window opening at 17:55:16 until 18:00:35. | Same log |
| How long did Apple's step take? | At most 5m19s from the window opening. Click time wasn't recorded. 1.9G on disk. | Same log, `du -sh` |
| What does canceling look like? | Canceling at the first start made the installer exit at 17:54:45 with no tools present. Whether Ben had started the download isn't recorded. | Same log |
| Can the Homebrew command be pasted? | Apple's alert "Possible Malware, Paste Blocked", with a red "Paste Anyway" and a default "Don't Paste". It names "chat agents" among the usual sources. | Ben, in Terminal. [Apple's note](https://support.apple.com/127377) |
| Is the `.pkg` trustworthy? | Signed "Developer ID Installer: Patrick Linnane (927JGANW46)", notarized, Homebrew 7.0.12, sha256 `a9825310…441bd`. | `pkgutil --check-signature`, `shasum -a 256` |
| Does the `.pkg` install Homebrew? | Yes, with Apple's tools already present. `brew` runs, owned by `admin`. | `open`, then `brew --version` |
| Does it need the PATH step? | No. A login shell finds `brew`, through `/etc/paths.d`, with no `.zprofile`. A non-login ssh shell doesn't find it. | `zsh -lc "which brew"` |
| Does a Brewfile work on a new account? | Not by itself. `brew bundle` taps, then refuses all six: "Refusing to load formula … from untrusted tap". | `brew bundle --file=Brewfile` |
| Does a direct install work? | Yes. `brew install bensyverson/tap/job` trusts that formula by itself. | Same machine, tap untrusted |
| Does the Brewfile work after `brew trust`? | Yes. All six poured, in 9 seconds including the trust step. | `brew trust bensyverson/tap`, then `brew bundle` |
| Are they bottles? | agents, job, contextfill, sleepy, peep and woodcase all report `poured_from_bottle` true, with no Go on the machine. Each ran and shows team QJ3Q58XVEE. | `brew info --json=v2`, `--version`, `codesign -dv` |

## What this means

- **The `terminal` screen needs replacing.** Asking our audience to click through a malware warning is wrong even though it would work. The agent should download Homebrew's `.pkg`, verify it and open it. The new words are Ben's call.
- **`SETUP.md` must run `brew trust bensyverson/tap` before `brew bundle`.**
- **After `xcode-select --install`, the agent should run `open -a` on the installer,** so the "will pop up" wording stays true.
- **`wait clt` can rely on the git file.** One run supports this; it isn't proof.
- **`apple-wait` has no signal to start from** among those logged, so it should go unless the next run finds one.

## Still open, for a fresh clone

1. Does the `.pkg` work with no Command Line Tools, and does it bring them along? This could change whether Apple's step is needed, though Ben has ruled to keep it separate.
2. With Gatekeeper on, do the `.pkg`, the bottles and the binary still pass?
3. Run from an agent in the VM's own Terminal with the page in front: does Apple's window still open behind, and does `open -a` still raise it?
4. Canceling during the install itself, and canceling the `.pkg`.
5. Does `brew`'s file appear before the `.pkg` installer finishes? The logger here watched for the script's process, so it couldn't tell.
6. Is 927JGANW46 Homebrew's official identity? `SETUP.md` should pin it only once that's confirmed from Homebrew's own docs.
7. Does the page load with the network off? Run that with the network really off.
8. Do the threads run smoothly on the Mini's own display?
