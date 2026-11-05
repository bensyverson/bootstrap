# Handoff: the trial release is public (2026-11-05)

**Start here:** run `job orient`, then read [the VM runs](2026-11-05-vm-run-findings.md) ("Third run" and "What this means") and [the init findings](2026-11-05-init-existing-project-findings.md). This replaces [the VM handoff](2026-11-05-handoff-vm.md).

## Where things stand

- **`bensyverson/bootstrap` is public**, with release `v0.1.0`: a signed, notarized binary, its `.sha256`, a README and a trial `SETUP.md` that stops after the list screen. `scripts/release <X.Y.Z>` builds, signs, notarizes and pins, and `--publish` pushes, tags and uploads. The remote is SSH, because `gh repo create` set HTTPS, which git couldn't push to here.
- **`agents` 0.2.0 is released.** `init` refuses a `CLAUDE.md` of the project's own and points to the new `agents migrate`.
- **The tap's release now checks `PATH`:** after installing, every command must resolve to Homebrew's copy. The old `go install` and SwiftPM copies of `agents`, `job`, `contextfill`, `sleepy` and `peep` were removed from this Mac, and four scripts in `Jobs`, `nobedan` and `hirewell` now find `sleepy` on `PATH`. `nobedan`'s commit `e7e42a9` is **not pushed**: an earlier unpushed commit (`6795542`) that isn't from this session sits ahead of it. Ask Ben.
- **The page:** Ben approved `chat`, `brew-ask`, the cards' bob and colored shadow, and tighter titles. He filed a later idea: a click on the blob disturbs it from that point, then it settles (a job under `VNF9sG`).
- **`wait clt`** passed against a real Apple install (`started` on Agree, `arrived` 5m37s later), and no longer fires `started` twice.
- **The VM `clt3`** on the Mini is still running, with Apple's tools, Homebrew 7.0.12 and Claude Code (signed in as Ben). It's `admin`/`admin` at 192.168.64.2, and `~agent/tart-install/vm.exp "<command>" 192.168.64.2` runs a command in it. Stop it with `tart stop clt3` when it's no longer needed.

## In flight

Nothing. `DhuGEt` (the machine-neutral shared rules) was approved by Ben and pushed as `agents-md` `47fc9c1`; repos pick it up with `agents update`. `nobedan`'s unpushed commits stay unpushed (Ben: "No need to push nobedan").

## Next, in order

1. **The agent trial (`Y6LsGy`):** paste `https://github.com/bensyverson/bootstrap` into Claude Code, and into OpenCode, on an account with nothing set up, and record what each agent does. The clean VM is the natural place, but `clt3` now has everything installed, so clone a fresh one (see the VM handoff for the steps). Note that Claude Code's installer leaves `~/.local/bin` off `PATH` on a new Mac.
2. **Write the real `SETUP.md` (`W2yASt`)** from the findings:
   - Call everything Homebrew installs by full path during setup.
   - Don't add `brew shellenv` to the person's shell files.
   - Tell the person to open a new Terminal window before their next session.
   - Check for about 20 GB free before Apple's step, and raise Apple's dialog, which opens behind other windows.
   - Check Homebrew's `.pkg` is notarized as well as signed by 927JGANW46.
   - Run `agents migrate` when `init` asks for it.
   - Skip `job init` when `.jobs.db` exists, and pass `--as claude` or `--as opencode`.

   The `brew-ask` words are approved.
3. **The rest:** `GGbVZ5` (machine-wide settings: where `block-plan-mode.sh` ships from, and whether `jq` stays, are Ben's to decide), `9WWQm2` (the facts are in its note; offer `/remote-control` only to Pro and above with a claude.ai login), `t5FAqu`, `6jPdjy`'s remaining screens and real card contents, and the blob click.

## Traps

- **Never commit with `GIT_CONFIG_GLOBAL=/dev/null`** (see gotchas). Run git unsandboxed with the real config.
- **`spctl -a -t exec` rejects every bare tool;** use `-t open --context context:primary-signature` (see gotchas).
- **`scripts/release --publish` refuses a dirty tree,** and every `job` write dirties `.jobs/log`. Commit the log right before publishing.
- **Worktree cleanup:** `.claude/worktrees/agent-a2f1cffc921078ad6` is still locked by the harness; remove it once it's unlocked.
- **`agents-md`:** Ben's design-process edit is on its local branch `wip/design-process`.
