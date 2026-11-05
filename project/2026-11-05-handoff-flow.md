# Handoff: the server drives the setup (2026-11-05)

**Start here:** run `job orient`, then read [the server-driven flow](2026-11-05-server-driven-flow.md) (the design and its "Settled since" list) and [its build plan](2026-11-05-server-driven-build-plan.md). This replaces [the release handoff](2026-11-05-handoff-release.md).

## What changed this session

- **The agent trial ran in both apps** ([findings](2026-11-05-agent-trial-findings.md)). Claude Code's auto mode blocked the first run of the binary until the person said yes in the chat. OpenCode's free default model never ran `wait`, so the person hit the lost screen twice. Each click waited 8 to 10 s for the agent.
- **Ben reversed the core decision: the server, not the agent, drives the setup.** The agent asks for consent, verifies the binary, runs `bootstrap run`, loops on `bootstrap wait`, retells what it reads, and takes over on handoffs.
- **Built and merged under `CAwBVg`:**
  - the VM spike (`Fg5Kre`): every install step works from a detached process;
  - the flow core (`3rqKIp`, `internal/flow`);
  - the page's instant click response and "That's all for now." end screen (`XAPDKr`, approved live by Ben);
  - the `run`/`wait`/`resume` verbs and the first real runner (`VBA13e`).
- **Ben's rulings, all in the flow doc's "Settled since":**
  - never ask for the admin password;
  - no commit for the person, though `git init` stays;
  - build the `.gitignore` from what's known;
  - set Git's name and email for them, skippably;
  - the folder step opens the project part;
  - GitHub sign-in happens in the software;
  - new words are reviewed as rendered screens.

## Next, in order

1. **`6PL0Nz`:** wake the flow at once when the server records an answer, instead of the 200 ms poll (Ben asked why we poll). It's widened to also log a flow that stops early, which `wait` can't see today.
2. **`UyKEjV`, the real installers.** Its notes carry the details:
   - remove `StepCommit`;
   - stream `gh`'s one-time code to the page;
   - raise Apple's window with `open -a`;
   - check `agents init`'s source argument;
   - the Git identity screen;
   - the `.gitignore` by project type;
   - the GitHub signup and code screens.

   New words go to Ben as rendered screens. The placeholder screens `git-ask` and `settings` hold `TODO-WORDS`.
3. **`W2yASt`:** rewrite `SETUP.md` for consent, `run` and `wait`, plus the chat-only fallback. Its description was rewritten for the new premise.
4. **`QKSluk`:** release, then rerun the trial in both apps. Ask Ben before publishing.

## Traps

- **The pre-commit hook is local** (`.git/hooks/pre-commit`, not tracked). It now runs `gofmt` only over `go list ./...` folders, so agents' worktrees don't block commits.
- **A vanilla Tart VM is short of disk for Apple's tools;** see `gotchas.md`.
- **VMs on the Mini:** `spike` (192.168.64.4) and `clt3` are stopped, and both have Apple's tools and Homebrew. Delete them when no longer useful; `fresh` is older still.
- **The Mini's `ben` account now has OpenCode and tmux** from Homebrew. The copied Anthropic key was deleted.
- **Nothing from this session is pushed.**
- **Worktree cleanup:** `.claude/worktrees/agent-a2f1cffc921078ad6` is still locked by the harness from an earlier session.
