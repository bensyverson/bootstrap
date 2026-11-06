# Handoff: 0.3.0, bootstrap in the tap, and two clean-Mac passes (2026-11-06, evening)

**Start here:** run `job orient`, then read this. It replaces [the afternoon's handoff](2026-11-06-handoff-trial.md). The runs are in [the trial findings](2026-11-06-clean-mac-trial-findings.md), "Third run" and "Fourth run".

## What changed this session

- **bootstrap ships through the tap** (`wz9ORc`, closed). The tools step installs `bensyverson/tap/bootstrap` with no card and deletes `~/.bootstrap/bin/bootstrap`; a failure keeps the download. `next:` lines from the download name `/opt/homebrew/bin/bootstrap` once it's there (`detect.Bootstrap`). `scripts/release X.Y.Z --publish` now writes `internal/cli/version.go` and ends by running the tap's release. See [the upgrade plan](2026-11-06-bootstrap-upgrade-plan.md).
- **MIT license** added; the formula says so.
- **Words approved by Ben:** naming's "Skip"; done's "You can close this. When you want a project, ask {ai} to run bootstrap new."; the remembered default as the main button. No screen is a draft. `XbdzWw`, `VGCn9C` and `a6Q6ie` closed.
- **v0.3.0 released** and pushed (main and the tap). The bottle poured on grace, so `/opt/homebrew/bin/bootstrap` is installed there, as Ben wanted.
- **Two clean-Mac passes on clones of `trial-base`:** Claude Code (Opus 5.5) and OpenCode (free `big-pickle`) both reached done in about 9 minutes, and both moved onto Homebrew. The second-run check (`bootstrap new --yes` changes nothing) passed, so `UyKEjV` is closed.
- **New scripts:** `scripts/page-demo.sh` (the real page on a pretend Mac, for judging motion live) and `scripts/vm-snap.sh` (snapshots of what the setup can change, in a VM).

## Not yet released

Committed after v0.3.0 and **not pushed**: "Standard developer tools, including Git." (`977f655`), brew-ask held 2 s (`b789cac`), plain `bootstrap new <name>` for the person (`b789cac`), and the findings. They ship with the next release, which needs Ben's OK.

## Next

1. **The chat-only setup** with the download blocked (`W2yASt`'s `gjk`), on a new clone: `tart clone trial-base <name>`. Never run `trial-base` itself.
2. **`PjAKYq`:** skip the phone step in OpenCode. Check OpenCode's docs for a real equivalent first.
3. **`S2lZZq`:** teach every session that bootstrap makes projects, so done's line can say "just ask Claude to bootstrap a project". Who writes it is Ben's call.
4. **Ben is doing the `agents` work himself:** `zIOjF2` (`agents gitignore`), `1lz2aN` (`agents hooks install`) and `lOQp0x` (agents-md's declarations), all in agents' tracker. `agents` stays agnostic: modules declare `ignore:` and `pre-commit:` in their frontmatter, like `seeds`. Then bootstrap's `TE90Df` calls both commands in the rules step.
5. Release the unreleased changes above, with Ben's OK.

## VMs on the Mini

- `trial-base`: the clean base (Setup Assistant done, Remote Login on). Clone it, never run it.
- `trial-oc1`: the OpenCode pass, still running when this was written. Stop and delete it once nobody needs to look: `tart stop trial-oc1; tart delete trial-oc1`.
- Tips from the afternoon still hold. A VM's clipboard takes text over ssh only after someone has logged in at its screen. OpenCode's installer adds no PATH line. OpenCode's transcript is `~/.local/share/opencode/opencode.db`, readable with `sqlite3`, which ships with macOS.
