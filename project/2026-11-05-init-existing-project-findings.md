# What `agents init` and `job init` do to an existing project (2026-11-05)

Findings for `g0RIoY`. `SETUP.md` (`W2yASt`) should describe these tools as they actually behave, and the setup has to be safe in a folder that already holds files.

**Short answer:** no case lost or overwrote anything the user had. But when a project already has its own `CLAUDE.md` that isn't a link to `AGENTS.md`, `agents init` leaves it alone and still exits 0. Claude Code reads `CLAUDE.md`, so it never sees the rules and nothing reports the problem. That one needs a fix before `SETUP.md` is written.

## How it was run

`scripts/init-cases.sh <scratch dir>` builds each case in its own folder and then runs the same three commands the setup will run:

```
agents init github.com/bensyverson/agents-md
job init --as claude
job gitignore
```

It prints each folder before and after. The binaries are the released Homebrew builds, `agents 0.1.0` and `job 0.1.0` from `/opt/homebrew/bin`, not the `go install` builds in `~/go/bin`. The source resolved to `agents-md` `36e2a04`. `AGENTS_REGISTRY`, `AGENTS_CACHE`, `GIT_CONFIG_GLOBAL` and `XDG_CONFIG_HOME` all point into the scratch folder, so Ben's real registry stayed clean. The script fetches from github.com, which the sandbox blocked even with the domain allowed, so it ran unsandboxed.

**The identity flag.** `job init` refuses to run without `--as` (or `--strict`). The setup passes the agent's own name: `job init --as claude` in Claude Code and `job init --as opencode` in OpenCode (Ben, 2026-11-05).

## Cases

| Case | `agents init` | `job init` / `job gitignore` | Outcome |
|---|---|---|---|
| Empty folder, not a Git repo | Seeds the `AGENTS.md` head from the template, renders the modules, seeds `project/gotchas.md` and `project/backlog.md`, links `CLAUDE.md -> AGENTS.md`, registers. Exit 0. | Creates `.jobs.db` and `.jobs/local.json`, then writes a new `.gitignore`. Exit 0. Doesn't print the `.gitignore` hint, because the folder isn't a repo. | Fine. Nothing in either tool needs Git. |
| Empty Git repo | Same as above. | Same, plus the hint to run `job gitignore`. | Fine. |
| Real `CLAUDE.md`, no `AGENTS.md` | Seeds `AGENTS.md` from the template, says `CLAUDE.md is not a link to AGENTS.md; left alone`, exit 0. | Fine. | **Rules never reach Claude Code.** `CLAUDE.md` is untouched, and the template head (with its `<placeholders>`) isn't built from it. |
| `CLAUDE.md` links to a file outside the project | Same: left alone, exit 0. | Fine. | **Same problem.** The shared file outside was untouched, which is correct. |
| `CLAUDE.md -> AGENTS.md`, with content | `AGENTS.md`'s content becomes the head. `already linked`. | Fine. | Fine. |
| Real `AGENTS.md` and a different real `CLAUDE.md` | `AGENTS.md` becomes the head, and `CLAUDE.md` is left alone. | Fine. | **Same problem.** The `CLAUDE.md` rules and the generated ones now sit in two files, and Claude Code reads only one of them. |
| `.gitignore` with no final newline | — | Adds a newline and a blank line, then its block. | Fine; the existing entries are intact. |
| `project/gotchas.md` already exists | Doesn't seed it, and keeps it byte for byte. | — | Fine. |
| Only `.claude/CLAUDE.md` | Links a root `CLAUDE.md -> AGENTS.md`. `.claude/CLAUDE.md` is untouched. | — | Fine; Claude Code reads both. |
| `.jobs.db` already exists (identity `someone`) | — | `job init` fails: `.jobs.db already exists. Use --force to overwrite`, exit 1. The database and its identity are untouched. | Safe, but the playbook must treat this exit 1 as "already done". It must never retry with `--force`, which would wipe the tasks. |
| Second run over a finished setup | Every step says `already …`, exit 0. | `job init` exit 1 as above. `job gitignore` says the entries are already there, exit 0. | Fine. `agents init` can be rerun; `job init` can't. |

## What this means for `SETUP.md`

- **Check before `job init`**, rather than reading its exit 1: if `.jobs.db` exists, skip it. Never pass `--force`.
- **The `AGENTS.md` head from the template holds `<placeholders>`** (`<docs path>`, the overview, the docs list). The agent has to fill them from what it learns about the project, or ask.
- **Neither tool creates `local/`.** The setup still creates it and its `.gitignore` line, as the plan says.
- **Every init registers the folder** in the user's `~/.config/agents/repos`. That's harmless, and it's what lets `agents status --all` find the project later.
- **The `CLAUDE.md` cases** need the fix below first.

## Decided: a project that already has its own `CLAUDE.md`

This doesn't lose content, but it fails silently, and in an existing project it's the common case: someone who has used Claude Code before usually has a `CLAUDE.md`. The fix goes in `agents` itself (Ben, 2026-11-05):

- **`agents init` errors** when `CLAUDE.md` exists and isn't a link to `AGENTS.md`, before it writes anything, and points to `agents migrate`.
- **`agents migrate` handles a real `CLAUDE.md`.** With no `AGENTS.md`, it renames the file to `AGENTS.md` and links `CLAUDE.md` to it. When both exist, it appends `CLAUDE.md`'s content under a marker at the end of `AGENTS.md`, then replaces `CLAUDE.md` with the link. Nothing is lost, and pruning stays a human step, as it already is for `init`.
- **A `CLAUDE.md` that links to a file outside the project is refused** by both commands, with an explanation. Copying the file in would quietly cut the project off from the shared original, so a human decides.

So `SETUP.md` runs `agents init`, and if init points to `agents migrate`, runs that and then init again.

**Released as `agents` 0.2.0** (`agents` job `5L3o4E`, commit `42298e7`, a signed and notarized bottle). The poured bottle refused a lone `CLAUDE.md` and then migrated it. Rerunning the cases with a build of it (`AGENTS_BIN=<build> scripts/init-cases.sh <scratch> claude_only claude_symlink_out both_differ`): init refused all three before writing anything, and named `agents migrate` for the two real files. `migrate` then moved `claude_only`'s file to `AGENTS.md`. For `both_differ` it put `CLAUDE.md`'s text under the marker at the end of the head, above the first region. Init then succeeded on both, and both ended with the moved text in the head. `migrate` refused `claude_symlink_out`. The table above records `0.1.0`.
