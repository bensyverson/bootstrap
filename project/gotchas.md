# Gotchas

Project-specific traps that cost real time and that no general rule predicts. Read at session start; append when you hit one.

- **Fix at the source when you can.** A gotcha is a bug report on our tooling, not a permanent fact — if it can be fixed in code, file it in `job` and fix it instead of recording it here.
- **Delete anything that becomes obvious, gets fixed, or stops recurring.** Keep this list short; a long list is one nobody reads.
- **Feedback about `AGENTS.md` itself** — a rule that was wrong, misread, or cost time — goes here too, prefixed `rule:`. It is harvested when the shared rules are reviewed.

Format: one dated H2 headline, then one paragraph. If an entry needs more than that, it's a finding — write it as a dated doc under `project/` and link it from the paragraph.

---

## 2026-11-05 Headless Sleepy never ticks animation frames

`requestAnimationFrame` never fires in `sleepy`, and a probe that awaits it hangs. Web animations also sit at their first frame, so a page that fades in screenshots as blank. Draw once directly and test stills through a link that opens a screen at rest (`#tools-jobs` in the prototype). Live motion can only be judged in a real browser.

## 2026-11-05 `spctl -a -t exec` rejects every bare command-line tool

It assesses apps, so a signed and notarized `bootstrap` comes back "rejected (the code is valid but does not seem to be an app)", which reads as a failed notarization. Check a bare binary with `spctl -a -vvv -t open --context context:primary-signature <file>` and look for `source=Notarized Developer ID`. See [the binary plan](2026-11-05-bootstrap-binary-plan.md), "Verifying the download".

## 2026-11-05 A vanilla Tart VM is about 0.5 GB short for Apple's tools

`ghcr.io/cirruslabs/macos-tahoe-vanilla` has 18.7 GB free, and Apple's Command Line Tools refuse to start below 19.18 GB. `tart set --disk-size` doesn't help: the new space lands behind the recovery partition, which SIP won't let `diskutil` move or delete. Clear `~/Library/Caches`, `~/Library/Application Support/com.apple.wallpaper`, `/private/var/folders/*/*/C`, then run `sudo log erase --all` before starting Apple's step. That reached 19.25 GB. See [the VM runs](2026-11-05-vm-run-findings.md), "Fourth run".

## 2026-11-05 `pkgutil --check-signature` says "invalid signature" inside the Bash sandbox

Run sandboxed, it reports `Status: invalid signature` (exit 1) for Homebrew's good package, because it can't reach the trust service. With the sandbox off, the same file says signed, notarized and `Developer ID Installer: Patrick Linnane (927JGANW46)`. Check a signature with the sandbox off. The flow refuses the sandboxed answer, which is right. See [the seams doc](2026-11-05-bootstrap-seams.md#the-real-installers-uykejv-2026-11-05).

## 2026-11-06 Hiding `~/.gitconfig` from the sandbox commits under the wrong email

Inside the sandbox, git can't read `~/.gitconfig` and stops with a fatal error. Setting `GIT_CONFIG_GLOBAL=/dev/null` gets past that, but git then guesses an identity from the hostname, and a commit made that way carries `ben@<host>` instead of Ben's GitHub noreply address (5c5b4ec, 2026-11-06). That override was once the handoffs' tip for running `agents` in the sandbox, and a commit made with it nearly reached a pushed release of `agents`. Run `git commit`, and any release script that commits, outside the sandbox with the real config, and check `git log -1 --format='%ae'` before pushing.

## 2026-11-06 A subagent can't sync its worktree with main

The briefing's sync step (`git merge --ff-only main`) fails inside the sandbox because git can't read `~/.gitconfig`, and the classifier refuses the agent's retry with the sandbox off. The agent then works on whatever base its worktree started from. Commit before dispatching so the harness's worktree starts at HEAD, tell the agent the sync will fail and to skip it, and merge with the squash at integration, which handles a stale base.
