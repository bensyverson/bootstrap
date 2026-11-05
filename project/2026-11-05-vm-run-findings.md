# VM runs: Homebrew's paste is blocked, its `.pkg` works, and Apple's step has a click signal (2026-11-05)

**The headline:** on macOS 26.6.2, Apple blocks a pasted Homebrew command with a malware warning, so the `terminal` screen can't stay as designed. Homebrew's signed `.pkg` works on a bare Mac, and our tools pour from it even without Apple's tools. Apple's step still matters, because without it plain `git` is Apple's placeholder. The step has a signal for the moment someone clicks Agree, so `apple-wait` stays. Two runs, described below: the first VM (`clean`) and a fresh clone (`fresh`).

> **Correction, 2026-11-05:** the first version of this doc said `apple-wait` had no signal to start from and should go. The second run found one, below.

**The setup:** Tart VM `clean`, cloned from `ghcr.io/cirruslabs/macos-tahoe-vanilla`, on the Mac Mini. macOS 26.6.2, arm64. Ben clicked over Screen Sharing and commands ran over ssh from the Mini with `~agent/tart-install/vmrun.exp`.

**Before anything ran:** `xcode-select -p` exited 2, `/Library/Developer/CommandLineTools` didn't exist, `pkgutil --pkgs | grep -iE "CLTools|Xcode"` found nothing, and there was no Xcode or Homebrew.

## First run (`clean`)

### How this VM differs from a real new Mac

- **Gatekeeper is off.** `spctl -a -vv -t install` printed `override=security disabled`. This run can't show that Gatekeeper lets anything through.
- **Passwordless `sudo` came with the image.** `/etc/sudoers.d/admin-nopasswd` allowed it. I removed that file before the Homebrew attempts, and `sudo -n true` then said a password was required.
- **The account's display name is "Managed via Tart".** The installer filled that in as the username, and Ben's first login there failed. It went through after I suggested typing `admin`, so a real Mac's dialog isn't shown to have this problem. `dscl . -authonly admin admin` confirmed the password.
- **It runs macOS 26, not the macOS 15 floor.**
- **Apple's step was started over ssh** (`launchctl asuser 501 xcode-select --install`), not by an agent in the VM's own Terminal.

### What happened

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
| Is `brew` on the PATH? | In a login shell, yes: `/etc/paths.d/homebrew` appeared at install time, with no `.zprofile`. In a non-login shell, no: `zsh -c` doesn't find it. | `zsh -lc "command -v brew"`, `zsh -c "command -v brew"` |
| Does a Brewfile work on a new account? | Not by itself. `brew bundle` taps, then refuses all six: "Refusing to load formula … from untrusted tap". | `brew bundle --file=Brewfile` |
| Does a direct install work? | Yes. `brew install bensyverson/tap/job` trusts that formula by itself. | Same machine, tap untrusted |
| Does the Brewfile work after `brew trust`? | Yes. All six poured, in 9 seconds including the trust step. | `brew trust bensyverson/tap`, then `brew bundle` |
| Are they bottles? | agents, job, contextfill, sleepy, peep and woodcase all report `poured_from_bottle` true, with no Go on the machine. Each ran and shows team QJ3Q58XVEE. | `brew info --json=v2`, `--version`, `codesign -dv` |

## Second run (`fresh`)

A new clone of the same image. Before anything ran: Gatekeeper on (`spctl --global-enable`, then `spctl --status` said `assessments enabled`), passwordless `sudo` removed, and the same checks found no Apple tools, Xcode or Homebrew. A logger ran unprivileged, as an agent would: state changes each second in `~/vmrun/signals.log`, a per-second timeline of network, CPU and the frontmost app in `~/vmrun/raw.log`, and `log stream` for Apple's installer, `softwareupdated`, Installer and `installd`. Apple's step was again started over ssh.

| Question | What happened | How |
|---|---|---|
| Does Gatekeeper accept the `.pkg`? | Not in this VM: `rejected`, source "Notarized Developer ID", and syspolicyd logged "Error checking with notarization daemon: 3", even after a reboot. The identical file was `accepted` on Ben's Mac (27.0.1) and on the Mini (26.6.2). Cause unknown. | `spctl -a -vv -t install`, sha256 `a9825310…` on all three |
| Does the `.pkg` work with no Apple tools? | Yes. Ben reported no warning and no mention of Apple's tools, and none were installed. | Ben, then `xcode-select -p`, `pkgutil --pkgs` |
| Does `brew`'s file come before the installer closes? | Yes: file 18:29:01, receipt 18:29:02, Installer gone 18:29:07. | `~/vmrun/signals.log` |
| Can Homebrew install our tools with no Apple tools? | Yes. It tapped, pulled in its own Git 2.56.0, and poured all six tools, which ran. | `brew install`, `--version` |
| Is Homebrew's Git the one that runs? | No. `which -a git` lists `/usr/bin/git` first, which is Apple's placeholder until Apple's tools arrive. | Login shell |
| How much space does Apple's step want? | Ben saw "19.18 GB is needed and only 18.7 is available". `softwareupdated` logged 18697367552 available. It went ahead at 19.54 GB free, after Homebrew was removed. It uses about 1.9 GB once installed, and the logged downloads total 1.77 GB (15 packages). So about 15.5 GB of the requirement is headroom Apple keeps, not space the tools use. That's inferred by subtraction; Apple doesn't document it. `softwareupdated` logged "lack of free disk space (19181109136 required, 18161135616 available); asking CacheDelete for space". | Ben; `df -k /`; the `Starting download of … (Size:…)` lines in `log stream` |
| Is there a signal for the click? | Yes, at Agree. At about 18:37:09 two folders (`047-91568`, `140-17812`) holding `CLTools_…pkg` files appeared in `/Library/Updates`. `softwareupdated` logged "Starting download of CLTools_…", and network input rose to about 14 MB/s. The Install click only showed an installer CPU blip, and the installer also blipped at 18:32:41 with no click, so CPU isn't a signal. | `ls -A /Library/Updates`, `log stream`, `netstat -ib` |
| What did the disk refusal look like? | Only `index.plist` appeared, with no folders and no download. | Same |
| What happens at the end? | Download finished 18:39:54. Git's file and `xcode-select` 18:42:47, receipt 18:42:48. `softwareupdated` logged "Removing 047-91568, 140-17812", and the folders were gone at 18:42:52. Ben clicked Done at 18:44:21. 5m38s from Agree to finished. | Same |
| Does Apple's window open in front? | No, Finder stayed in front both times, and `open -a` brought it forward both times. | `lsappinfo front` |
| Do the tools work with Apple's tools present? | Yes. After `installer -pkg`, `brew bundle` poured all six as bottles, and `git` is Apple's. Free space afterward: 16.72 GB. | `brew info --json=v2` |

What this run can't show: Gatekeeper's verdict on the `.pkg` (the VM's own check failed), and on a file a browser downloaded, since everything here came by `curl`, which doesn't mark files for Gatekeeper. `brew trust` was remembered after Homebrew was removed and reinstalled, so the second Brewfile didn't re-test trust.

## What this means

> **Built, 2026-11-05:** `wait clt`'s three events, `brew-ask` in place of `terminal`, and the disk row in `detect`. See [the seams doc](2026-11-05-bootstrap-seams.md).

- **The `terminal` screen needs replacing** with Homebrew's `.pkg` (Ben, 2026-11-05). The new words are Ben's call.
- **`SETUP.md` must run `brew trust bensyverson/tap` before `brew bundle`.**
- **After `xcode-select --install`, the agent should run `open -a` on the installer.** In all four starts it opened behind.
- **Keep Apple's step.** Homebrew and our tools don't need it, but without it, plain `git`, and probably `python3`, `make` and `clang`, are placeholders that pop up Apple's window at a random moment. Since nothing depends on it, it doesn't have to finish before Homebrew starts.
- **`wait clt` has signals at both ends.** Agree: a new folder holding `CLTools_` files in `/Library/Updates`, so `apple-ask` can move to `apple-wait`. Finished: Git's file. Canceled: Apple's installer process exits with no Git (first run). Two runs support the finished signal, one supports the click signal.
- **Catch the common errors, not every edge case** (Ben, 2026-11-05). The common ones here are canceling and too little disk space. A folder an earlier, interrupted try left behind could fire the click signal early. The cheap guard is to look only for folders that weren't there before Apple's step started, and it's optional.
- **Check free space up front.** Apple's step wanted about 19 GB free. `bootstrap detect` should report it, so the agent can explain before the person gets stuck.
- **`SETUP.md` can't stop on `spctl` alone,** since it rejected a good file in this VM. Check the hash and `pkgutil --check-signature`.

## Still open

1. Gatekeeper's real verdict, on a Mac that isn't this VM image, including a file a browser downloaded.
2. Run from an agent in the VM's own Terminal with the page in front: does `open -a` still raise Apple's window?
3. Can an agent that was already running when Homebrew installed find `brew`? Its PATH was set before the install, so it probably can't, and `SETUP.md` should call `/opt/homebrew/bin/brew` by full path unless this shows otherwise. The same goes for the tools it installs.
4. Is 927JGANW46 Homebrew's official identity? `SETUP.md` should pin it only once that's confirmed from Homebrew's own docs.
5. Does the page load with the network off? Run that with the network really off.
6. Do the threads run smoothly on the Mini's own display?
7. Not to be tested, per Ben: canceling partway through Apple's install or the `.pkg`.
