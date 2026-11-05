# Handoff: the VM runs reshaped the Homebrew step (2026-11-05)

> **Superseded, 2026-11-05,** by [the release handoff](2026-11-05-handoff-release.md).

**Start here:** run `job orient`, then read [the VM runs](2026-11-05-vm-run-findings.md) and [the seams doc](2026-11-05-bootstrap-seams.md). This replaces [the build handoff](2026-11-05-handoff-build.md), whose review commands and earlier rulings still hold.

## Where things stand

- **On `main`, unpushed, suite green.** There's still no public repo.
- **A pre-commit hook now exists** in `.git/hooks/pre-commit`: gofmt, `go vet`, `go test ./...` and `agents check`. It's local to this checkout and isn't committed.
- **Built this session:** the `chat` screen; `brew-ask` in place of `terminal`, with Ben's words and the Homebrew card; `brew-wait` removed; `wait clt` returns `started`, `arrived` or `canceled`, and `wait brew` returns `arrived` or `canceled`; `detect` reports free space. The installer names and the `/Library/Updates` sign are in `catalog/tools.json`.
- **The rules pin** moved to `agents-md` `36e2a04`, which puts the shell-variable rule in core. Fifteen other registered repos already had uncommitted `AGENTS.md` changes from before this session; nothing here touched them.
- **The VM:** `fresh` on the Mini, stopped. It now has Apple's tools, Homebrew and all six tools, so a test from nothing needs a new clone (`tart clone ghcr.io/cirruslabs/macos-tahoe-vanilla:latest <name>`, then `spctl --global-enable` and remove `/etc/sudoers.d/admin-nopasswd`). `clean` is deleted. Login is `admin`/`admin`. `~agent/tart-install/fr.exp "<command>"` runs a command in `fresh` over ssh (IP 192.168.64.3; check with `tart ip fresh`). The VNC port and password change each run and are in `~/tart-install/run-fresh.log`.

## Next, in order

1. **Ben's live review** of what he hasn't seen: `show chat`, and `show brew-ask` with the Homebrew card. Commands are in [the build handoff](2026-11-05-handoff-build.md), "Ben's review". Two card ideas from `6jPdjy`'s notes aren't built: a slow bob while installing, and the buttons' colored shadow on cards.
2. **Check `wait clt`'s `started` and `canceled` against a real Apple install.** The tests use a fake folder and process list. Needs a new clone and Ben clicking.
3. **Settle before `SETUP.md` (`W2yASt`):** is 927JGANW46 Homebrew's official signing team? Can an agent that was running before Homebrew installed find `brew`? Until that's shown, `SETUP.md` should call `/opt/homebrew/bin/brew` by full path and run `brew trust bensyverson/tap` before `brew bundle`. It mustn't stop on `spctl` alone, because the VM's `spctl` rejected a good `.pkg`.
4. **Trial release (`vHAiCj`):** ask Ben before publishing. Copy `/Users/ben/git/goodall/.env` to the Mini for OpenCode only when the trial needs it, never into the repo or a VM, and never print its values.
5. **The rest:** `g0RIoY` then `W2yASt`, remaining screens and real cards (`6jPdjy`), `GGbVZ5`, `DhuGEt`, `t5FAqu` (now the optional `new-project` skill; its hint goes on the page and in the chat, and the words are still Ben's), `9WWQm2`.

## Rulings made this session

All are recorded in the docs and job notes they rule on: the `chat` screen never shows the lost screen and keeps the server up; the new-project shortcut is an optional skill named `new-project`; use Homebrew's `.pkg` rather than the pasted command; the `brew-ask` words; keep Apple's step; keep `apple-wait`, entered on the Agree signal; delete `brew-wait`; the Command Line Tools line is "Apple’s tools, which include Git."; the Homebrew card goes on `brew-ask`; catch common errors, not every edge case; no cancel tests.

## Traps

- **Sleepy's pre-commit suite** was killed at its 5-minute cap twice, then passed in about 160s. Load was high at both kills, but also high during a pass, so the cause isn't established. `scripts/release`'s `check` has no such cap, so a stall there would hang rather than fail.
- **Tap script gaps:** a new formula needs a placeholder `v0.0.0` url, and a formula fix after a step 5 failure needs a manual push. Neither is documented in `--help`.
- **Worktree cleanup:** `.claude/worktrees/agent-a2f1cffc921078ad6` and its branch were locked by the harness when this was written. The work is already squashed into `648a2f3`, so remove both once unlocked.
- **Woodcase renders need images:** `Tests/WoodcaseTests/Fixtures` has no `images/` folder. `local/viewer-demo/images` does.
- **The page's card contents** for Homebrew, Git, Contextfill and Woodcase are still made up or illustrative.
- **`agents-md`:** Ben's design-process edit is on local branch `wip/design-process`.
- **Sandbox:** running `agents` sandboxed needs `AGENTS_CACHE` in scratch, `GIT_CONFIG_GLOBAL=/dev/null` and `github.com` allowed.
