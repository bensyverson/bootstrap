# `bootstrap upgrade`, and bootstrap in the tap (2026-11-06)

Ben asked for a `bootstrap upgrade` that brings the tools up to date and runs `agents update` in the current repo, with an `--all`. This records his rulings and the work.

## Rulings (Ben, 2026-11-06)

- **bootstrap ships through the tap too.** It gets a Homebrew formula in `bensyverson/tap`, bottled, signed and notarized like the other tools. Once Homebrew is installed, the setup moves bootstrap onto it, so `brew upgrade` updates bootstrap like everything else. It has no updater of its own. The first run still downloads the pinned, checked file in `SETUP.md` to `~/.bootstrap/bin`, because Homebrew isn't there yet.
- **Plain `bootstrap upgrade`:**
  - runs `brew upgrade bensyverson/tap/<formula>` for each tool from the catalog found under `/opt/homebrew/`, bootstrap included. This reuses the tools step's `upgradeTool`, so the rule "a copy built elsewhere is the person's own" holds here too;
  - runs each tool's `hook install` again (it changes nothing when current);
  - runs `agents update` in the current folder when it holds `.agents.yaml`.
- **`bootstrap upgrade --all`** runs `agents update --all` over every repo `agents` has registered (`~/.config/agents/repos`), instead of just this one.
- **Rules that move are left uncommitted.** `agents update` shows a diff of what changed and re-renders. A repo that refuses to overwrite a hand-edited region is reported with its path, and the exit status says so; nothing is forced.

## Rulings on shipping it (Ben, 2026-11-06, later)

- **One command does both releases.** `scripts/release X.Y.Z --publish` states the version in `internal/cli/version.go` (the tap's `scripts/release` reads a tool's version from its source), commits it with SETUP.md's pin, tags and uploads, then runs the tap's release from `../homebrew-tap` for the formula and bottle. An unstamped build now reports the last release's version, not `dev`.
- **The move happens in the tools step, with no card.** After the tools, the step runs `brew install bensyverson/tap/bootstrap` (or `brew upgrade` when Homebrew's copy is there) and deletes `~/.bootstrap/bin/bootstrap`. If brew fails, as it will until the formula is published, the run notes `kept ~/.bootstrap/bin/bootstrap` and goes on with the download. It never hands off.

## What was built (`wz9ORc`, 2026-11-06)

- `detect.Bootstrap` names the live copy: Homebrew's once it's there, the download until then. The done handoff uses it.
- `next:` lines from the downloaded copy name `/opt/homebrew/bin/bootstrap` once that exists, so a `wait` started before the move points the agent at the copy that's left. A build run from anywhere else (a checkout) is still named by its own path.
- `Formula/bootstrap.rb` in the tap: built from source with Go, `depends_on macos: :sequoia` like the other Go tools, the version stamped by `-ldflags`. It points at v0.2.0 until the first release rewrites it. The repo has no license file yet, so the formula has no `license` line.
- `scripts/release` step 0 (state the version) and step 9 (the tap's release); `scripts/test-release` covers both.

## Open questions for the builder

- What `upgrade` prints and its `--json` shape, following `project/agents/cli-design.md` and the other verbs (`next:` lines by path).
- Where the moved binary lives once Homebrew has it (`/opt/homebrew/bin/bootstrap`), and what happens to `~/.bootstrap/bin/bootstrap`: a link to Homebrew's copy, or removed, with `next:` lines naming whichever runs.

  > **Settled, 2026-11-06 (Ben):** delete `~/.bootstrap/bin/bootstrap` once Homebrew's copy is installed. From then on `next:` lines name `/opt/homebrew/bin/bootstrap`.
- Whether `upgrade` refuses while a setup holds the setup slot (`session.Store.LockSetup`). It probably should, since both install.

```yaml
tasks:
  - title: Ship bootstrap through the tap
    desc: |
      Ben, 2026-11-06 (project/2026-11-06-bootstrap-upgrade-plan.md): add a bootstrap formula to bensyverson/tap, bottled, signed and notarized like the other tools, with scripts/release.conf keys following contextfill's. Once Homebrew is installed, the setup installs bootstrap from the tap and the rest of the run uses that copy. Releases push tags and upload bottles, so they need Ben's OK. Decide what happens to ~/.bootstrap/bin/bootstrap.
    criteria:
      - brew install bensyverson/tap/bootstrap pours a signed, notarized bottle
      - "After the setup, the bootstrap that next: lines name is Homebrew’s copy, shown by a test"
  - title: Add bootstrap upgrade
    desc: |
      Ben, 2026-11-06 (project/2026-11-06-bootstrap-upgrade-plan.md): `bootstrap upgrade` runs brew upgrade for each catalog tool under /opt/homebrew/ (reusing the tools step's upgradeTool), runs each tool's hook install again, and runs `agents update` in the current folder when it holds .agents.yaml. `--all` runs `agents update --all` over every registered repo instead. Moved rules stay uncommitted; a repo that refuses a hand-edited region is reported and makes the exit non-zero. Library first (internal/flow or a new package), thin verb in internal/cli.
    blockedBy: [Ship bootstrap through the tap]
    criteria:
      - bootstrap upgrade upgrades only Homebrew's copies, shown by a test
      - In a managed repo it runs agents update; outside one it says so and skips it
      - "--all runs agents update --all, shown by a test"
      - Help and the primer document upgrade
```
