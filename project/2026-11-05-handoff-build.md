# Handoff: the binary is built, and the page awaits Ben's review (2026-11-05)

**Start here:** run `job orient`, then read [the binary plan](2026-11-05-bootstrap-binary-plan.md) and [the seams doc](2026-11-05-bootstrap-seams.md). This replaces [the earlier handoff](2026-11-05-handoff.md).

## Where things stand

- **Built and on `main`, unpushed:** the core, server, CLI and real page. `go test ./...` passes. There's no public repo yet.
- **`IjcOYy` (the page) is closed:** Ben reviewed it live on 2026-11-05 and approved the motion. The commands below still work for later reviews.
- **The release agent was still running when this was written:** `contextfill` 0.1.3, `job` 0.1.0 and `agents` 0.1.0 were reported released, unverified by the integrator. Sleepy and Peep were still to do, and Sleepy's change was staged in `/Users/ben/git/SleepyHollow`, waiting for this Mac's load to fall, because its WebKit tests deadlock under load. `J2FJEI` and `GB3eQp` hold the results. Check `job show` on both, and `git -C` each repo, before assuming anything.
- **The VM:** Tart 2.41.1 is in `~agent/tart-install` on `agent@sally.local`, cloning `macos-tahoe-vanilla` into a VM named `clean`. Progress is in `~/tart-install/clone.log`.

## Next, in order

1. **Integrate the release agent** if it hasn't been: verify its bottles, then close `J2FJEI` and `GB3eQp`.
2. **Add the chat screen** (approved by Ben): title "Let's talk in {app}.", line "{ai} has a question." It has no buttons, and it must not show the lost screen, because the agent stops running commands while waiting for a reply in the chat. Red first, then regenerate samples with `go test ./internal/screen -update`.
3. **Done: Ben approved the page.**
4. **VM run (`Y6LsGy`):** confirm the VM really has no Command Line Tools, then run Apple's step and Homebrew. Ben clicks Install over Screen Sharing. It also has to answer whether a dependable signal exists for the "your turn" screens. If so, use `apple-wait` and `brew-wait`; if not, delete them. And it should check whether Homebrew's file appears before its installer finishes. Also try the threads on the Mini's own display.
5. **Trial release (`vHAiCj`):** ask Ben before publishing. He wants everything that can be approved locally approved first. Copy `/Users/ben/git/goodall/.env` to the Mini for OpenCode only when the trial needs it, never into the repo or the VM, and never print its values.
6. **The rest:** remaining screens and real cards (`6jPdjy`), `g0RIoY` then `W2yASt`, `GGbVZ5`, `DhuGEt`, `t5FAqu`, `9WWQm2`.

## Ben's review

Run in a real terminal, because the sandbox blocks `open`:

```
cd /Users/ben/git/bootstrap
go build -o /tmp/bootstrap-review ./cmd/bootstrap
export BOOTSTRAP_DIR=/tmp/bootstrap-review-state
/tmp/bootstrap-review serve --agent claude-code
```

For previews, keep that tab open and change `/?token=` to `/preview?token=` in a second tab. Click a state, then use the left and right arrow keys. To drive it live, use `/tmp/bootstrap-review show welcome`, then for example `show tool jobs --status downloading`, `show tool jobs --status installing`, `show done`. Finish with `/tmp/bootstrap-review stop`.

Things for him to judge that the prototype didn't have: after a click, the other choices dim to 45% and the object speeds up until the agent moves on. Pressing Enter in the name field doesn't submit, which matches the prototype.

## Rulings made this session

All are recorded in the plan docs and job notes: the agent drives; the page is only a viewer and a channel for decisions; the "Whoops!" wording; folder detection; `~/Projects` as the default, with `~/git` as the other suggestion; Apple's step stays separate; stay on "your turn" screens until the install finishes; git-friendly names; three real renders on the Woodcase card; the Jobs card keeps its screenshot; macOS 15 as the floor; PixelPeeper is MIT; no `say` verb for now ([backlog](backlog.md)).

## Traps

- **Worktree cleanup:** `.claude/worktrees/agent-a2f1cffc921078ad6` and its branch were locked by the harness when this was written. The work is already squashed into `648a2f3`, so remove both once unlocked.
- **Woodcase renders need images:** `Tests/WoodcaseTests/Fixtures` has no `images/` folder. `local/viewer-demo/images` does.
- **The page's card contents** for Homebrew, Git, Contextfill and Woodcase are still made up or illustrative.
- **`agents-md`:** Ben's design-process edit is on local branch `wip/design-process`.
- **Sandbox:** running `agents` sandboxed needs `AGENTS_CACHE` in scratch, `GIT_CONFIG_GLOBAL=/dev/null` and `github.com` allowed.
