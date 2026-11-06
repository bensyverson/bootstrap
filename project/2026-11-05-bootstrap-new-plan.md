# `bootstrap new`: the setup from the command line (2026-11-05)

**Why.** Ben will set up new project folders himself, and agents should be able to run the setup without a page (`t5FAqu`). Ben, 2026-11-05: the CLI and the page should only ask about what isn't set up yet; the CLI asks interactively unless the answers come as flags; options are opt-out, so a plain verb with no flags gets the defaults; the project type is detected from existing files, else "Something else". He left the parser to us and suggested the name `bootstrap new`.

## Decisions

- **The verb is `bootstrap new [<name>]`.** It runs the same setup the page runs, in the foreground, with no server, page or browser. A starting folder that's already a project is set up in place, and `<name>` isn't needed; otherwise `<name>` makes `~/Projects/<name>`.

  > **Corrected 2026-11-05 (`gpvHd3`):** `~/Projects` is now only the default. `--in <folder>` puts the project in `<folder>/<name>`, and the folder picked, here or on the page, is remembered in `~/.bootstrap/preferences.json` for the next setup. See [the seams](2026-11-05-bootstrap-seams.md), "Tidying `new`, and where projects go".
- **One flow, two ways of asking.** `internal/flow` stays the only copy of the steps, their checks and their order. It gains a seam for asking the person: the page (today's show-and-wait on the session log) or the terminal. Library and thin executable: the CLI only wires the terminal side in. A second copy of the steps in `internal/cli` is the thing to avoid.
- **The existing parser, not Cobra.** `internal/cli` already handles flags, switches, help and errors that teach, and Cobra would be the first outside library (a dependency needs Ben's OK, and this doesn't need one).
- **Every question has a default and a flag.** With a terminal, each question is asked with its default ready, so Enter takes it. With `--yes`, or with no terminal (an agent), the defaults are taken without asking. A question with no default (a project name, outside a project) fails with exit 2, naming the argument or flag that answers it.

  > **Corrected 2026-11-05 (Ben, `OWv6U2`):** no terminal no longer takes the defaults. With no terminal (or `--json`) and no `--yes`, `new` refuses with exit 2 before touching anything, listing each question no flag or earlier answer settles and the flag that answers it. A terminal decides only between asking and refusing, never between asking and installing; `--yes` is the only way to take defaults unasked. Each `--no-X` also gained a `--X` that takes back an earlier no.
- **Defaults are opt-out.** Every tool in the catalog is kept unless `--skip <tool>` says otherwise. Git is used, and the identity is set from GitHub when signed in. Machine settings are turned on. The project type is detected: `package.json` means a website, and `Package.swift` or an `.xcodeproj` means an app. Without either, it's "Something else". Live yet defaults to "not yet". Phone is a page-only question, so the CLI prints a one-line hint instead.

  > **Superseded 2026-11-06:** Ben dropped the live-yet question, since nothing used its answer. A new project gets `stage-build`, a project set up in place gets no stage, and `new` has no `--live` flag.
- **Handoffs end the command.** A trouble handoff prints what happened and the evidence, as `wait` does, and exits 4 with a `next:` line. Running `new` again picks up, since every step checks before it acts. Steps that need the person's clicks (Apple's window, Homebrew's installer, GitHub's code) print what to do and wait, as the page does.

## The leaf

Imported 2026-11-05 under `SddU2d`, beside `t5FAqu`, which keeps the preference and the skill; the tracker holds the current version.

```yaml
tasks:
  - title: Run the setup from the command line with bootstrap new
    ref: new
    desc: |
      Why: Ben sets up project folders himself and wants it quick, and agents should be able to run the setup with no page (t5FAqu). His rulings are in project/2026-11-05-bootstrap-new-plan.md, "Decisions": ask only what isn't set up yet, ask interactively unless the answers come as flags, opt-out defaults so `bootstrap new` with no flags just works, and detect the project type from existing files, else "Something else".

      What: `bootstrap new [<name>]` runs the whole setup in the foreground, with no server, page or browser. With a terminal, it asks each open question with its default ready (Enter takes it). With --yes, or with no terminal, it takes the defaults, and a question with no default fails with exit 2 naming the flag or argument. Each question has a flag (for example --making, --skip <tool>, --no-git, --no-github, --no-identity, --no-settings); the builder picks the exact set and documents it in --help. A trouble handoff prints the evidence as `wait` does and exits 4 with a next: line; running new again picks up.

      Already decided: one flow. internal/flow keeps the only copy of the steps; it gains a seam for asking (the page's show-and-wait, or the terminal). The existing parser in internal/cli, not Cobra. Defaults are listed in the plan doc. Follow project/agents/cli-design.md.

      Seams: internal/flow (run.ask, run.asked, run.show, and the session store it writes to), internal/cli (a new command beside run, wait and resume), internal/detect (project type from files). The page flow must behave exactly as before: its tests are the guard.

      Out of scope: the "skip the tour next time" preference and the new-project skill (the other half of t5FAqu); any change to the page.

      Open, the builder's call: whether `new` writes a session log, and where (the page's log in ~/.bootstrap must not be confused by a run with no server); how progress screens become terminal lines.

      Risks: the flow assumes a page in places, such as the chat screen during a handoff and the lost screen; a terminal run must not wait on a page that isn't there.
    criteria:
      - On a pretend Mac, `bootstrap new pine-bakery --yes` with no terminal sets up the project with the defaults and no page, shown by a cmd/bootstrap test
      - A second `bootstrap new` in the same folder changes nothing and asks nothing, shown by a test
      - With no terminal and no name outside a project, new fails with exit 2 naming the argument, shown by a test
      - The project type comes from package.json or Package.swift when present, and "other" otherwise, shown by tests
      - The page flow's existing tests pass unchanged
```

> **Settled 2026-11-06 (Ben), from [the clean-Mac trial](2026-11-06-clean-mac-trial-findings.md):** later setups remember the person's answers. A decline on the page stays declined in a later `bootstrap new --yes`. An explicit flag (`--github`, `--no-github`, …) overrides the remembered answer and becomes the new one. `bootstrap prefs` lists what's remembered, and `bootstrap prefs reset <name>` forgets one, so the next setup asks again. `--yes` never starts a step that waits on a person; it skips it and says so. Answers live in `~/.bootstrap/preferences.json`, beside `projects_in`. Leaf `XbdzWw`.
