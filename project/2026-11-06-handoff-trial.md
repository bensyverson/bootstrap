# Handoff: the clean-Mac trial and its fixes (2026-11-06, afternoon)

**Start here:** run `job orient`, then read this. It replaces [this morning's handoff](2026-11-06-handoff.md). The trial's record is [the findings doc](2026-11-06-clean-mac-trial-findings.md).

## What changed this session

- **`SETUP.md`** took three cold reads' worth of fixes (90a1bb3, 36c7a7c) and the trial's (ca2aae9). `W2yASt`'s `Hl7` and `x7l` passed; only `gjk` is left, the chat-only setup run for real.
- **Stack detection** (895f3f8, af74f97): `go.mod`, Python's files and Swift's add their rules and `.gitignore` lines (Ben's ruling, in [the setup link plan](2026-11-05-setup-link-plan.md)).
- **v0.2.0 released** and pinned, main pushed (0d7f6ea). Everything after it is **not pushed**: Ben asked to test before the next push and release.
- **The trial** (f5fd140): run 1 on a Cirrus clone stopped at Apple's step for disk; run 2 on a VM from Apple's installer reached done on Sonnet 5.5. Clicks drew the next screen within ~70 ms; auto mode needed only the chat yes.
- **Fixes from the trial, merged:**
  - Apple's step raises the installer best-effort and never hands off over it (6374e0f, `h43hpn` closed).
  - README and `SETUP.md`: `curl` the raw file, not `gh`/`git`; no `osascript`; a click-cured handoff keeps looping (ca2aae9, `3Dz3wE` closed).
  - The page holds a new working screen 300 ms, and questions clear the percentage at small windows (6359e29; `OaDn4i` closed, `a6Q6ie` open for Ben's live look).
  - Remembered answers with `bootstrap prefs`, and "Just the tools for now" (ae005ff; `XbdzWw`, `VGCn9C` open for Ben).
- **Rulings recorded:** remembered answers, updated by flags and `bootstrap prefs` ([`bootstrap new` plan](2026-11-05-bootstrap-new-plan.md)); just the tools keeps the machine-wide steps ([server-driven flow](2026-11-05-server-driven-flow.md)); delete `~/.bootstrap/bin/bootstrap` once Homebrew's copy is in ([upgrade plan](2026-11-06-bootstrap-upgrade-plan.md)).
- **Filed:** `vumAwF` (contextfill's caveats and OpenCode `PATH`), `wiKWPk` (the settings screen's promise in OpenCode).

## Needs Ben before the next release

1. **Approve the draft words** on `naming` ("Just the tools for now") and `done` (the just-tools line): `bootstrap serve`, then `/preview#naming.claude-code` and `/preview#done.tools.claude-code`. `scripts/release` refuses drafts.
2. **See the remembered default:** on the page, a remembered choice becomes the main button, even "Not now". Approve or ask for another look (`XbdzWw` criterion `sth`).
3. **Judge live:** no flash after the tool list or naming; a slow step's working screen arriving 300 ms later; words rising over the object at short windows (`a6Q6ie`).

## Next

1. **The base VM.** `trial-base` was building on the Mini from Apple's installer (100 GB disk) at the end of the session: check `~agent/tart-install/create-base.log` and `tart list`. Boot it (`tart run --vnc-experimental --no-graphics trial-base`, the VNC URL lands in the log), tunnel the port from `grace` (`ssh -N -L <port>:127.0.0.1:<port> agent@sally.local`), and have Ben do Setup Assistant: account `admin`/`admin`, no Apple Account, Remote Login on. Then **stop it and never run it again**: every pass is `tart clone trial-base <name>`. Keep Tart's IPSW cache this time.
2. **Release** after Ben's approvals (`scripts/release 0.3.0 --publish`, with his OK), which pushes main.
3. **Passes from clones:** Claude Code again (to see the fixes), OpenCode on its free model, and the chat-only setup with the download blocked (`gjk`). Then rerun the second-run check (`bootstrap new --yes` changes nothing) for `UyKEjV`'s `GEk`.
4. **`wz9ORc`**, the bootstrap formula: Ben is waiting for it rather than installing by hand.

## How the trial was run

- From `grace`, everything goes over ssh to `agent@sally.local` (the Mac Mini); Tart is `~agent/tart-install/tart.app/Contents/MacOS/tart`, and `~agent/tart-install/vm.exp "<cmd>" <ip>` runs a command in a VM as `admin`. Run these with the sandbox off.
- **Never run `python3` or `git` in a fresh VM** to inspect it: both are Apple's stubs and pop the developer tools dialog. Copy files out with `vm.exp "cat …"` and read them on `grace`.
- Tart's VNC has no shared clipboard: put text on the VM's clipboard with `vm.exp "echo admin | sudo -S launchctl asuser 501 sudo -u admin sh -c 'pbcopy < file'"`.
- A VM sees Screen Sharing as a mouse with natural scrolling on, so scrolling is reversed until it's turned off in the VM's Mouse settings.
- Logs from both runs are in `local/trial-2026-11-06/` (private).
