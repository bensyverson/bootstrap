# Building the server-driven flow (2026-11-05)

This stages the work behind [the server-driven flow](2026-11-05-server-driven-flow.md), which Ben approved on 2026-11-05. That doc holds the shape: the agent asks for consent, verifies the binary and runs `bootstrap run`; the server works through the setup order and shows it on the page; the agent loops on `bootstrap wait`, retells the events, and takes over on handoffs. Read it before any leaf here.

## Order, and why

1. **The spike comes first, and in parallel with the core.** The core is pure Go over a fake `Runner`, so it doesn't wait on the VM. The installers do wait on it: if `gh` or Apple's installer won't run from a detached process, those steps become handoffs, which changes what the installer leaf builds.
2. **The page leaf is independent** and can run any time. It fixes what Ben saw in the trial, and the server-driven page needs both fixes anyway.
3. **The core, then the verbs, then the installers.** The verbs adapt the core, and the installers plug into it.
4. **`SETUP.md` (`W2yASt`) is rewritten once the verbs exist**, so it describes real output. Its description is edited to the new premise when this plan is imported.
5. **The stage closes on a release and a rerun of the agent trial,** in both apps.

## Decisions that bind every leaf

- **The server drives; when unsure, it hands off and never guesses** (Ben, 2026-11-05, [the flow doc](2026-11-05-server-driven-flow.md)).
- **Errors that teach:** every handoff names what happened, the evidence, and the command for each way forward (`project/agents/cli-design.md`).
- **The page answers every click at once** (Ben, 2026-11-05).
- **Every step checks before it acts,** so a second run changes nothing.
- **Library plus thin CLI:** the decisions live in `internal/flow`, and the verbs only wire them.

## Tasks

Imported on 2026-11-05 under `VNF9sG`; the tracker holds the current version.

```yaml
tasks:
  - title: Make the server drive the setup
    ref: container
    desc: |
      Ben approved on 2026-11-05 that the `bootstrap` server, not the agent, drives the setup: "I don't think we can rely on the latency and inconsistency of the model driving each step." The agent trial (project/2026-11-05-agent-trial-findings.md) showed a full agent turn per click, 8.5 to 9.5 s, and a weak model that never ran `wait`, so the person hit the lost screen. The design is project/2026-11-05-server-driven-flow.md, and the order is in project/2026-11-05-server-driven-build-plan.md.
    children:
      - title: Check each install step runs from the detached server
        ref: spike
        desc: |
          Why: the server will run every install itself, as a detached background process started by the agent's shell. Some steps may need a terminal or a GUI session, and the installer leaf builds differently for each step that can't run detached: that step becomes a handoff to the agent.

          The question, per step: run from a process started the way `bootstrap serve` starts its server (re-launched in the background, no terminal), does it work? The steps: `xcode-select --install` and raising Apple's window with `open -a`; downloading Homebrew's latest `.pkg` from its GitHub releases, checking its signer (927JGANW46) and notarization, and opening it; `brew trust bensyverson/tap` and `brew install` by full path; `agents init`, `job init`, `git init`; and `gh auth login --hostname github.com --web --git-protocol https --skip-ssh-key`, reading the one-time code from its output and getting past its "Press Enter" prompt with no terminal.

          Where: a fresh Tart clone on the Mini (`tart clone ghcr.io/cirruslabs/macos-tahoe-vanilla:latest <name>`, then `spctl --global-enable` and remove `/etc/sudoers.d/admin-nopasswd`); project/2026-11-05-handoff-vm.md has the access details. Run the steps from a small throwaway Go program or a `nohup`/`setsid`-style launch that matches how the server detaches. Its code is disposable. Ben clicks Apple's and Homebrew's windows. A GitHub sign-in needs an account: ask Ben which one to use, and never record its token.

          Findings go in project/2026-11-05-vm-run-findings.md as a new section, or a new dated doc, and as a note on the installer leaf.
        criteria:
          - A findings section records, for each step, the command, how it was launched, and whether it worked detached
          - "gh's web sign-in is shown working from a detached process, or the reason it can't is recorded"
      - title: Make the page answer every click at once and end cleanly
        ref: page
        desc: |
          Why: in the agent trial (project/2026-11-05-agent-trial-findings.md) every click left the old screen up for 8 to 10 s, and after `bootstrap stop` the page sat on the answered list screen looking busy forever. With the server driving, most clicks will get the next screen at once, but anything that waits (a handoff, a slow step) still needs an answer the person can see.

          What: a click acknowledges itself within a frame. The answered screen moves into a waiting state, following the no-hard-cuts rule (every change animates on the shared 1.4s S-curve, and the object never moves; see "The interface" in project/2026-11-05-setup-link-plan.md), and it holds until the next screen arrives. When a session ends on a screen that isn't settled, the server shows an internal end screen, like `lost`, with Ben's words: title "That's all for now.", line "You can close this and go back to {app}." It settles the blob.

          Already decided: those words (Ben, 2026-11-05). The page's own waiting words are not decided: propose them, and get Ben's OK before shipping. Show both states as sleepy stills, and Ben judges the motion live.

          Seams: internal/page/files/page.js (`answer`, `draw`, `setAnswered`), internal/page/files/live.js (`ended`), internal/session/session.go (`Frame`, `Lost`), internal/screen/defs.go (the screen table, where `lost` is the model for an internal screen). Headless sleepy never ticks animation frames (project/gotchas.md), so test stills through a link that opens a screen at rest.

          Out of scope: the blob reacting to a click (xwxkLY).
        criteria:
          - A test shows an ended frame on an unsettled screen carries the internal end screen with Ben's words
          - Sleepy stills show the answered-waiting state and the end screen
          - Ben approves the waiting state's words and motion live, recorded in a note
      - title: Build the flow core
        ref: core
        desc: |
          Why: the setup order moves from SETUP.md's prose into code the server runs, so it can't be skipped or reordered and can be tested. This is the library the verbs and installers plug into.

          What: a new package, internal/flow. The setup is a list of steps as data, each with a check (already done?), an act, the screens it shows, and the answers it needs from the page. A step's outcome is done, skipped, needs the person (a page screen), or a handoff to the agent with a reason, the evidence and the ways forward. Every outside command goes through a `Runner` interface, so tests use a fake and no test touches the Mac. The flow writes every step, event and handoff to the session log, as `show` does today, so `status` and `wait` read it with no server connection.

          The order is in project/2026-11-05-server-driven-flow.md, "What the server does": Apple's tools, Homebrew, tools, then the project part (name and folder, Git, GitHub, stage questions, agents init, local/, job init), machine settings, phone, done. The tools come from the person's picks on the list screen and catalog/tools.json. The folder step makes ~/Projects/<name> unless the starting folder is already a project, and never moves files.

          Already decided: when unsure, hand off, never guess (an unexpected exit code, a step far past its expected time). Every step checks before it acts. Library plus thin CLI.

          Seams: reuse internal/detect, internal/screen, internal/session and the clt and brew watchers behind `wait clt` and `wait brew`. The server (internal/server) gains a way to run the flow; keep its existing page, token and lifetime rules.

          Out of scope: the CLI verbs (next leaf) and the real installers (the installer leaf); use the fake Runner here.

          Risks: the session log's format is shared by the CLI, the server and the page. Change it in one place, with one struct both sides import.
        criteria:
          - Tests with a fake Runner walk the whole flow from nothing to done, and a second walk over the finished state changes nothing
          - Tests show each unexpected result (non-zero exit, timeout) becomes a handoff with a reason and ways forward, never a guess
          - A test shows a starting folder that is a Git repo is set up in place and no files move
      - title: Add the run, wait and resume verbs
        ref: verbs
        blockedBy: [core]
        desc: |
          Why: these are the agent's whole interface to the server-driven flow. A weak model must be able to drive it by following each output's `next:` line alone.

          What: `bootstrap run --agent <app>` starts the server and the flow, detached, and returns at once with `next: bootstrap wait`; a second `run` joins the flow already running. `bootstrap wait` blocks until the flow needs the agent, or its budget runs out (exit 3, "still working, run wait again"), and either way prints the events since the last wait as rows written to be retold. `bootstrap resume --retry|--skip` hands back after a handoff. `stop`, `status` and the read verbs stay. Examples of the rows and of a handoff are in project/2026-11-05-server-driven-flow.md, "The shape".

          Already decided: the house CLI rules in project/agents/cli-design.md: terse rows with `--json` on request, the shared exit codes, a bare `bootstrap` as a primer, every error and ack naming the next command, and tests that drive the binary and read the remedy. Keep wait's default budget under Claude Code's 10-minute tool limit, and check OpenCode's.

          Seams: internal/cli; `show` and `wait answer|clt|brew` stay for the handoff screens, tests and the preview. Out of scope: SETUP.md.

          Open, the agent's call: exact flag names, and whether `run` replaces `serve` or wraps it.
        criteria:
          - "Tests drive the binary through run, wait (events and a budget timeout), a handoff, resume --retry and resume --skip, asserting output and exit codes"
          - Every handoff type has a test that reads its remedy
      - title: Wire the real installers
        ref: installers
        blockedBy: [core, spike]
        desc: |
          Why: the core runs on a fake Runner; this leaf makes each step act on a real Mac.

          What: each step's check and act for real, called by full path (/opt/homebrew/bin/brew, never `brew` by name, because a running session can't find it after the .pkg installs): Apple's tools (about 20 GB free first, `xcode-select --install`, raise the window with `open -a`, the clt signals); Homebrew (find the latest .pkg, check it's signed by 927JGANW46 and notarized, open it, wait for brew); `brew trust bensyverson/tap` before installing the picked tools; Git (repo, .gitignore, first commit); GitHub (`github.com/signup` when the person has no account, then `gh auth login --web`, with the one-time code shown on the page, and `gh auth status`); the project (`agents init`, `agents migrate` when init asks, local/ and its .gitignore entry, `job init --as <agent>` unless .jobs.db exists, `job gitignore`); machine settings through each tool's own `hook install` (GGbVZ5, blocked on those tools shipping it).

          Already decided: everything in project/2026-11-05-vm-run-findings.md and project/2026-11-05-init-existing-project-findings.md, which these steps follow. Never add `brew shellenv` to the person's shell files. Don't stop on spctl alone (a VM rejected a good .pkg).

          The spike decides which of these run detached and which become handoffs; follow its findings, and edit this description if they change the premise.

          Seams: internal/flow's Runner. New screens for GitHub (signup, the one-time code) go through internal/screen, with words for Ben to approve; 6jPdjy owns the remaining screens and real cards.
        criteria:
          - A VM run from a fresh clone reaches done with every picked tool installed, and a second run changes nothing
          - The Homebrew step refuses a .pkg with the wrong signer, shown by a test
      - title: Release and rerun the agent trial
        ref: trial
        blockedBy: [verbs, installers, page, W2yASt]
        desc: |
          Why: this closes the stage. The reversal was made to fix what the first trial showed (project/2026-11-05-agent-trial-findings.md): one agent turn per click, and OpenCode's free model never running `wait`.

          What: a signed release with `scripts/release`, SETUP.md pinned to it (W2yASt rewrites SETUP.md for run and wait first), then the same trial: paste the bare link into Claude Code and into OpenCode on its free default model, in a fresh VM or account, start them from the GUI session so the browser opens, and record what happens in a dated findings doc. Ask Ben before publishing. Check whether one up-front yes in the chat gets past Claude Code's auto mode for a program that installs things.
        criteria:
          - In both apps, the bare link reaches done with no lost screen, recorded in a findings doc
          - "Each click shows the next screen, or the waiting state, within a second, shown by the session log's timestamps"
          - Whether auto mode needed more than the first yes is recorded
```
