IMPORTANT: As you implement features, keep the documentation in `project/` and `SETUP.md` up to date; update README.md only when a doc file is added or new users must know.

## Overview

Bootstrap lets someone paste one link into Claude Code or OpenCode and get Ben Syverson's way of working: `agents` and the `agents-md` rules, `job`, Git, the supporting tools, and the `project/` and `local/` folders. It is for Mac users who are comfortable pasting into an agent, including people with no Homebrew or GitHub account yet. There is little code here: the repo is mostly a README an agent reads first, a playbook it follows, and a `Brewfile`; the tools themselves live in their own repos and ship through `bensyverson/homebrew-tap`.

**The `local/` folder rule lives here, not in `agents-md`** (Ben, 2026-11-05). The setup creates `local/` and its `.gitignore` entry; `local/` is private and never committed.

## Documentation

- [README.md](README.md) — the page a pasted link lands on (not written yet)
- [project/](project/) — dated plans, findings and decisions; the current plan is [the setup link plan](project/2026-11-05-setup-link-plan.md)
- [Setup page prototype](project/2026-11-05-setup-page-mock/prototype/) — the approved look and motion, in one page; `build.py` makes `local.html`; the [mock](project/2026-11-05-setup-page-mock/) shows the options tried
- [Clean Mac research](project/2026-11-05-clean-mac-research.md) and [what's stock on macOS](project/2026-11-05-stock-macos-runtimes.md) — what's settled, and where the rest gets tested
- [project/backlog.md](project/backlog.md) and [project/gotchas.md](project/gotchas.md) — work parked, and traps to know

<!-- agents:begin core@cfc560 -->
## Working rules

**Understand the why.** If the goal behind a request isn't clear, ask before solving — beware the XY problem.

**Diverge, then converge.** First brainstorm options (create choices), weigh them against the user's goals, recommend one (make choices), confirm, then execute.

**Ambiguity.** If the *code* could go several ways, choose the idiomatic one for the language. If the *requirement* is ambiguous or the question is architectural, stop and ask — don't decide.

**Dependencies.** Avoid them unless re-implementing would be unreasonable; ask before adding one; each is security and maintenance surface.

**TDD, strictly red/green.** Write tests for every case and every new method first, watch them *all* fail, then implement. A test that is green during red tests nothing — remove or rewrite it. If an existing test must change to pass because the behavior or expectation has changed, explain why clearly. Every bug fix starts with a regression test.

**Plans and tasks live in `job`.** Open every session with `job orient` (no arguments), then read `project/gotchas.md` — while reading, prune it: delete any entry that's now fixed, obvious, or a general rule, marking it `rule:` first if it's general. Don't use Plan Mode or ad-hoc todo lists.

**Don't tour the codebase.** Start from the README and the docs (an Explore agent is fine); dig only where the task leads — once you have a specific need, read as much as that need requires.

**Scripts.** Analysis tooling goes in `scripts/` so it can be re-run — check there before writing one.

**Critique before declaring done.** Re-read the original request: is the need actually met? Do lint and tests pass? Are docs updated? What would an expert flag? Fix serious flaws before reporting.

**Tidiness.** No stray files in the repo root; delete transients, and file valuable artifacts (reports, scripts) where they belong.

**Documentation.** Keep the project docs current as you build. Touch the README only when a doc file is added or new users must know.

**Gotchas.** When a project quirk costs you time and no rule predicts it, append it to `project/gotchas.md`. If a rule in this file was wrong or misled you, record that there too, prefixed `rule:`.

**Where these rules come from.** The marked regions are generated and shared across repos via a CLI tool named `agents`; don't edit inside them. If a rule here is wrong or cost you time, say so in `project/gotchas.md` prefixed `rule:`; that is how shared rules get reviewed.

**Local rulings.** A repo-local ruling, or an override of a shared rule, lives in the project-owned head of `AGENTS.md`, above the generated regions — say plainly that it overrides, and link a dated project doc for the why.

## A session

- **Start from the handoff doc in `project/`, if the last session left one, and `job orient`.** A handoff is current only if nothing has been committed since it (`git log` after its date); an older one is history, not a starting point. A handoff the user pastes wins.
- **On the main thread, read `project/agents/delegation.md` in full before you begin, so whether to delegate is a choice, not a default.** A subagent skips this: its brief is its scope, and it doesn’t delegate further.
- **Unless the user says they're around, assume they aren't.** Make the call yourself when it's safe and a two-way door, queue the rest, and report every decision at the end. While they're around, bring questions as they come up, including ones an agent raises mid-flight.
- **Before starting, say what needs the user's eyes.**
- **Keep working while there's work; once past your context line, start nothing new.** The main thread's line is 33%, a subagent's 40% (read with contextfill; never estimate). The line is where wrapping up begins: finishing what's in flight and handing off happen after it, however far past it that takes. A delegating main thread: delegation.md ▸ Carving the work.

## Git

- Offer to commit when a unit of work is complete and accepted. Rebase onto upstream; ask on real conflicts, explaining the conflict in plain terms first.
- Commit all uncommitted files together — later changes usually depend on earlier ones, and a half-working state helps nobody. Never amend.
- The subject completes "This commit…": present-tense verb first — "Adds…", "Fixes…", "Retires…". Detail goes in the body.
- Pass the message with `-F <file>`, not inline `-m`; the shell interprets `-m` first. Same for `job`: `note`, `done`, `add` and `edit` all take `-F <file>` (`-F -` reads stdin).
- Pre-commit hooks run the formatter and tests. Run them yourself first (see the stack rules).
- Never pipe a gating command (`git commit … | tail`) — the pipe swallows its exit status, so a following `&&` runs even after a failure.
- Before pushing a repo you haven't been working in, check what's ahead of upstream (`git log @{u}..`). Push only commits you made, or ask.
<!-- agents:end core -->

<!-- agents:begin principles@7a5b19 -->
## Principles

Defaults, not laws. When we break one, we do it consciously and say so in the report and the docs.

- **Pragmatism.** Builders, not purists. Practical choices that serve the near-term goal and protect the long-term one.
- **Eat the frog.** No band-aids. Given an easy-but-compromised path and a correct one, take the correct one; fix problems at the source. Keep YAGNI in mind, but when a need is obvious, don't underdeliver.
- **Composability.** Simple, strong components composed into systems — never a monolith.
- **Library + thin executable.** Core logic in a library; the app or CLI is a light consumer, so the core can be reused elsewhere. An adapter that holds a decision rather than wiring one is a bug.
- **Decoupling.** Tight coupling makes testing, debugging and refactoring hard — separate concerns. Separating a model, its storage and its UI is the everyday case: databases and UI frameworks change; today's web app may grow a CLI or mobile app.
- **Just enough abstraction.** One layer around an LLM provider is prudent; a `TextGenerationProvider` above it is not.
- **Readable file sizes.** Aim for files a reader can hold in their head (a few hundred lines; ~400 is the comfortable ceiling). Past ~2k lines, navigation degrades and errors accumulate; splitting also makes functionality discoverable by filename.
- **Comments say why, not what.** Doc comments state *what* concisely; other comments only explain the non-obvious. No change history in comments. Most code needs none.
- **Strongly typed.** Prefer enums, named constants and config over magic strings and numbers; prefer typed structs over dictionaries, even for wire types. Two packages exchanging data across a serialization seam share **one** struct that both import, never a hand-written twin on each side — the type checker cannot see across encode/decode, and two definitions drift. Given a bool and a typed constant, take the typed constant: a bool named for one consequence gets reused to gate the others until it means several things, so name the underlying *fact* as a type and let the behaviors follow.
- **Previews.** Give each UI component a way to render in its various states — a SwiftUI `#Preview`, a demo page, a story — the foundation for tests and for human review.
- **Async by default.** Keep the app interactive during heavy work; surface loading and error states. On the web, prefer progressive enhancement over full reloads.
- **Event streams where they fit.** Append-only logs are auditable, undoable, and time-travelable.
<!-- agents:end principles -->

<!-- agents:begin docs@7ba2fd -->
## Documentation practice

- **Plans, findings, designs and decisions go in `project/` as dated documents** (`YYYY-MM-DD-title.md`) — the written history of the project. They are point-in-time records: correct an earlier one *in place*, as a marked block quote, rather than silently editing a number or leaving a stale claim standing.
- **Every figure names the tool and flags that reproduce it.**
- **Work decided against goes in `project/backlog.md`**, not into silence: one dated H2 per item — what it is, why it's parked, and *what would un-park it*. Nothing there is scheduled or blocking; active work lives in `job`. Check it before proposing something that sounds novel.
- **When a finding overturns a premise, edit the premise.** Readers act on the title and opening; a correction appended underneath doesn't reach them.
- **A wrong documented cause is worse than none** — it stops the next reader looking. Correcting one means saying it was wrong, not quietly rewording.
- **Open the note before you cite it**, and check whether a recorded ruling has been superseded before passing it on.
- **Every repo has a README; if none exists, write one (delegate it if you can).** It is tight: the project's name and a one-line description a non-technical reader understands (6th-grade reading level); one short paragraph of what it is; how to install or consume it; a crisp Quick Start with an example or two; links out to the specific docs for anything more; authorship and license at the end.
- **The head of `AGENTS.md` lists where the docs live; keep that list current.**
<!-- agents:end docs -->

<!-- agents:begin index@8cf2c7 -->
## Situational instructions

These files carry instructions for specific situations. When one applies, read the file before acting and follow it.

| Situation | File |
|---|---|
| Before measuring anything, verifying a claim, or writing down a cause | `project/agents/evidence.md` |
| Before filing or claiming work in job, and when running as a subagent | `project/agents/jobs.md` |
| Before writing a plan, importing a `tasks:` block, or detailing the leaves of work ahead | `project/agents/planning.md` |
| Before dispatching any subagent | `project/agents/delegation.md` |
| The first time a tool call is refused or denied, and before briefing a subagent | `project/agents/harness.md` |
<!-- agents:end index -->
