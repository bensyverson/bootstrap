<!-- agents:begin review@63abe2 -->
# Code review

Independent review catches bugs that green tests miss. It covers code meant to be kept. A prototype declared up front — a `spike` label on its container, or a ruling in the `AGENTS.md` head — is exempt until it is built on or merged, and then all of it counts as unreviewed.

## When

Review the unreviewed work at the first of:

- **A job container closes.**
- **About 1,500 lines of production code** across the unreviewed leaves' commits, not counting tests, goldens, vendored or generated code.
- **A commit touches a persisted or wire format, `unsafe` or FFI code, locking, or deletion.** Review it before building on it.
- **Handoff.** Review what's left, or name the unreviewed commit range in the handoff for the next session to review first.

The unreviewed work is every closed leaf since the baseline without the `reviewed` label:

```sh
job ls --status done --leaves --without-label reviewed --since <baseline>
```

The baseline is a `Review baseline: <RFC3339 time>` line in the head of `AGENTS.md`. If there is none, add one with the current time; work closed before it is out of scope.

## How

- **One fresh reviewer per surface**, briefed as below, never given the author's report.
- **A finding counts once confirmed** by a probe that reproduces it or a trace with file and line.
- **File each confirmed finding as a leaf** carrying its reproduction, with `--found-in` naming the reviewed leaf.
- **Dispatch the fix if you can.** Unless you're past your limit, dispatch the fix immediately, so issues don't pile up.

## Briefing the reviewer

- **The code, by commit**, and where it lives. Not the author's account of it: a reviewer handed the reasoning tends to confirm it.
- **The goal as what must never happen, then the criteria.** The criteria say what the author tested, and the goal says what to try to break. A finding outside every criterion is the one the author couldn't see.
- **The stakes in both directions.** Name what the change could do wrongly and what it could fail to do. Fail-closed code has both: a wrong action, and a guard that never lets go.
- **The domain docs** that say how the system should behave, so the reviewer reasons about the system and not just the diff.
- **The failure classes this change invites**, chosen for it: races, error paths that fall through to a decision, tests that pass for the wrong reason, docs that claim more than the code does, plus the project's own traps.
- **The bar:** confirm each finding with a probe test it writes and runs, or a trace with file and line; mark anything else SUSPECTED, and keep the probe, because it becomes the fix's red test.
- **The report's shape:** each finding with severity and reproduction, then what was checked and found sound, so its coverage is visible. Say plainly that "nothing serious" is an acceptable answer, which takes away the pull to manufacture findings.

## Fixes

- **The reviewer doesn't fix.** It stays independent and verifies the fix.
- **The author fixes** while it is still running with room in its context. Otherwise a fresh agent does, assigned by file ownership (`delegation.md`), briefed with the finding and its reproduction.
- **Verify each fix against the reproduction**, by the reviewer or a fresh verifier.

## Mark it

```sh
job label add <id> [<id>...] with reviewed
```
<!-- agents:end review -->
