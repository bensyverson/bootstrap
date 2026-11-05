# Gotchas

Project-specific traps that cost real time and that no general rule predicts. Read at session start; append when you hit one.

- **Fix at the source when you can.** A gotcha is a bug report on our tooling, not a permanent fact — if it can be fixed in code, file it in `job` and fix it instead of recording it here.
- **Delete anything that becomes obvious, gets fixed, or stops recurring.** Keep this list short; a long list is one nobody reads.
- **Feedback about `AGENTS.md` itself** — a rule that was wrong, misread, or cost time — goes here too, prefixed `rule:`. It is harvested when the shared rules are reviewed.

Format: one dated H2 headline, then one paragraph. If an entry needs more than that, it's a finding — write it as a dated doc under `project/` and link it from the paragraph.

---

## 2026-11-05 Headless Sleepy never ticks animation frames

`requestAnimationFrame` never fires in `sleepy`, and a probe that awaits it hangs. Web animations also sit at their first frame, so a page that fades in screenshots as blank. Draw once directly and test stills through a link that opens a screen at rest (`#tools-jobs` in the prototype). Live motion can only be judged in a real browser.
