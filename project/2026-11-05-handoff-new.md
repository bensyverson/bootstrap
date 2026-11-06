# Handoff: installers, approved screens and `bootstrap new` (2026-11-05)

**Start here:** run `job orient`, then read [the server-driven flow](2026-11-05-server-driven-flow.md) ("Settled since", now 1 to 11), [the binary's seams](2026-11-05-bootstrap-seams.md) (newest sections: "Waking at once", "The real installers", "`bootstrap new`") and [the `bootstrap new` plan](2026-11-05-bootstrap-new-plan.md). This replaces [the flow handoff](2026-11-05-handoff-flow.md).

## What changed this session

- **`6PL0Nz` (166d36d):** a `session.Bell` wakes the flow and the server's page loop at once on an in-process write. The fallback read for other processes is now 1 s. A flow that stops early writes a `halt` event and the server exits, so `wait` reports `the setup stopped: …`.
- **`UyKEjV` (e34c10e), the real installers:**
  - no commit step;
  - `.gitignore` built from the making answer, in the rules step;
  - a skippable identity step;
  - Homebrew's `.pkg` checked by its first certificate plus notarization;
  - Apple's window raised;
  - gh's code streamed to the page;
  - GitHub sign-up.

  Still open: GEk, the VM run from a fresh clone.
- **`SETUP.md` rewritten** (e34c10e) for consent, `run`, the wait loop, handoffs, per-app notes and a 13-step chat-only setup. `TestTheChatOnlySetupFollowsTheFlowsSteps` keeps its steps in the flow's order.
- **Ben's live review of the screens** (da692c4, 8257d67):
  - `github-code` and `identity` sit low;
  - buttons follow the Mac order, main on the right and the way out on the left;
  - `live-yet` and `making` have no main button;
  - the words of all eight new screens are approved, so none is a draft.
- **Rules modules (5283596):** `agents add web` for a website, `swift swiftui` for an app, plus `stage-build` for a new project. A project set up in place gets no stage.
- **`l2isVc` (d6b73fa), `bootstrap new [<name>]`:**
  - it runs the flow in the foreground through a `flow.Person` seam;
  - opt-out defaults, with a flag per question and `--yes`;
  - each project keeps its own log in `~/.bootstrap/new/`.

  Its loose ends are `OWv6U2`.

## Waiting on Ben

1. **The consent wording in `SETUP.md` step 1.** Three options went to Ben: A, the full message (current); B, short; C, short plus "want the details first?". The recommendation is C, keeping "pick what gets installed".
2. **The defaults `new`'s builder chose beyond the plan** (in `OWv6U2`): GitHub signs in by default, Xcode opens the App Store, and `--agent` falls back to claude-code.

## Next

- **`W2yASt`:**
  - the README, which still says "trial";
  - a line in `SETUP.md` about `bootstrap new` for repeat users;
  - the dry read in a fresh session;
  - whether a Brewfile is still wanted (the flow installs from `catalog/tools.json`).
- **`QKSluk`:** release, then the trial in both apps. Before releasing, make `scripts/release` refuse any `screen.Drafts()` (none today). Ask Ben before publishing.
- **`gpvHd3`:** where new projects go becomes configurable (Ben, 2026-11-05): `--in <folder>` on `new`, a remembered choice in bootstrap's preferences file, and `~/Projects` or `~/git` on the naming screen. That preferences file is shared with `t5FAqu`.
- **`OWv6U2`**, and **`t5FAqu`** (the "skip the tour" preference and the `new-project` skill, which can now call `bootstrap new`).

## Traps

- **Don't push main before the release:** `SETUP.md` pins v0.1.0, which has no `run`. Nothing from this session is pushed.
- **A page from this session may still be running** (`./local/bootstrap`, a build in the ignored `local/`). Stop it with `./local/bootstrap stop`.
- **`pkgutil --check-signature` says "invalid signature" inside the Bash sandbox** (see `gotchas.md`).
- **`agents init` in a scratch folder registers it** in `~/.config/agents/repos`. Remove the line afterward.
