# The server drives the setup (2026-11-05)

**Status: the reversal is approved (Ben, 2026-11-05: "Yes, let's reverse this. I don't think we can rely on the latency and inconsistency of the model driving each step.").** It replaces a recorded decision in [the binary plan](2026-11-05-bootstrap-binary-plan.md), "The agent drives, and the binary only displays", which now carries a correction pointing here. The folder and GitHub answers under "Settled since" are proposals Ben raised as questions; confirm them before building.

Ben, 2026-11-05: "Could we just have the agent kick off the install process, and then have our Go server drive it until it exits?" and, on the costs, "We already got dinged by auto mode anyway. Maybe we just build in verbose output and/or great error messages that tell the agent how to recover."

## Why

[The agent trial](2026-11-05-agent-trial-findings.md) showed what it costs to have the agent drive every screen:

- **Each click waits a full agent turn,** 8.5 to 9.5 s on Opus 5.5, with the old screen still up.
- **A weak model breaks the loop.** OpenCode's free default model never ran `wait`, so the page fell back to the lost screen twice and the person had to type "continue" each time.
- **The flow lives in prose.** The order, the checks and the skip-if-done rules sit in `SETUP.md`, where an agent can skip or reorder them, and no test can catch it.

The earlier reason for the agent driving was that judgment stays with the agent, and that a flow in the binary would duplicate `SETUP.md`. The trial shows the flow is mostly not judgment. It's a fixed sequence of checks and installs, and that belongs in code that can be tested. The judgment that's left (consent, what went wrong, the chat) stays with the agent.

## The shape

The agent asks for consent, verifies the binary, and starts one run. The server then runs the setup, the page shows it, and the person's clicks go straight to the server. The agent loops on `wait`, narrates what it's told, and takes over only when the server hands it something.

| Command | Class | What it does |
|---|---|---|
| `bootstrap run --agent claude-code` | Act | Starts the server in the background and the flow with it. Returns at once, with `next: bootstrap wait`. A second `run` joins the flow already running. |
| `bootstrap wait` | Act | Blocks until the server needs the agent, or until its budget runs out. Either way it prints what happened since the last `wait`, as rows the agent can retell. |
| `bootstrap resume [--retry \| --skip]` | Act | Hands control back after a handoff. |
| `bootstrap stop` | Act | Ends the run. |

`detect`, `tools`, `screens` and `status` stay as they are. `show` stays for the screens an agent still drives (below), for tests and for the preview.

**`wait` is how the agent narrates.** Each return carries the events since the last one, one row each, written to be passed on:

```
event  person chose see-plan
event  installing homebrew: the person is clicking through Homebrew's installer
event  installed jobs 0.1.0 (bottle)
state  working  step=tools  4 of 9
next:  bootstrap wait
```

If the budget runs out, it exits 3 with the same rows and "still working, run wait again". The default budget stays under the shortest tool timeout we know of; Claude Code allows 10 minutes at most.

> **Corrected 2026-11-05 (`VBA13e`):** the limit that matters is the *default*, not the maximum. An agent that doesn't pass a timeout gets 2 minutes in both apps: Claude Code's Bash tool defaults to 120000 ms (its maximum is 600000 ms), and OpenCode's shell tool defaults to `DEFAULT_TIMEOUT_MS = 2 * 60 * 1_000` with no maximum (0 turns it off), changed by `OPENCODE_EXPERIMENTAL_BASH_DEFAULT_TIMEOUT_MS` (read with `gh api` from `anomalyco/opencode`, `packages/core/src/tool/plugin/shell.ts` and `packages/web/src/content/docs/cli.mdx`, on `dev`). A weak model won't pass one, so `wait` keeps the 90-second default (`session.DefaultWaitTimeout`). The rows as built are in [the seams doc](2026-11-05-bootstrap-seams.md#the-agents-verbs-vba13e-2026-11-05): `note` for a line to retell and `step` for a step's progress, rather than `event`.

**A handoff is an error that teaches** (`project/agents/cli-design.md`, "Errors that teach"). `wait` exits 0 with a `handoff` row that names what happened, the evidence, and the commands for each way forward:

```
handoff  trouble  tool=sleepy  reason=other
what:    brew install bensyverson/tap/sleepy exited 1
log:     ~/.bootstrap/steps/sleepy.log (last 20 lines below)
...
next:    read the log, fix what you can, then: bootstrap resume --retry
         or, to go on without it: bootstrap resume --skip
         or, to talk it over: tell the person in the chat; the page already says "Let's talk in {app}."
```

## What the server does, and what stays with the agent

The server runs the [setup order](2026-11-05-setup-link-plan.md#setup-order) as a state machine. Each step checks before it acts, so a second run changes nothing:

| Step | Server | Agent |
|---|---|---|
| Consent | | Explains what changes and asks before downloading anything. |
| Apple's tools | Disk check, `xcode-select --install`, raises Apple's window, watches the `clt` signals | |
| Homebrew | Downloads the `.pkg`, checks it's signed by 927JGANW46 and notarized, opens it, watches for `brew` | |
| Tools | `brew trust`, then each chosen tool by full path, one screen and card each | |
| Git | Repo, `.gitignore`, first commit, each asked on the page | |
| `gh` and GitHub | Asks; opens `github.com/signup` if needed; runs `gh auth login --web`, shows its one-time code on the page, waits for it to exit | |
| Project | Asks for a project name and makes `~/Projects/<name>` unless the starting folder is already a project; asks the stage questions, runs `agents init` (and `agents migrate` when asked), `local/`, `job init --as <agent>`, `job gitignore` | |
| Machine settings | Asks, then runs each tool's own `hook install` | |
| Phone | Asks | Explains `/remote-control` |
| Done | Shows done, with how to open the project | Summarizes in the chat, and tells the person to open a new Terminal window in the project folder |

> **Corrected 2026-11-05 (`UyKEjV`):** the Git row is now the repo only, asked on the page. There's no first commit (Settled since, 5), and the `.gitignore` is built without a question (7). A Git identity step follows GitHub (6). As built: [the seams doc](2026-11-05-bootstrap-seams.md#the-real-installers-uykejv-2026-11-05).

**Handoffs** are the rows with work on the agent's side, plus `trouble` and anything the server doesn't recognize, such as an unexpected exit code or a step that took far longer than allowed. When unsure, the server hands off; it never guesses.

**The lost screen** only matters during a handoff now. During installs, the page needs nothing from the agent, so a missing agent no longer stalls the person. If the agent is gone when a handoff comes, the page shows the lost screen as today, and "continue" picks up through `bootstrap status`.

**The page answers every click at once** (Ben, 2026-11-05), with a waiting state if the server is busy. When the server is driving, the next screen usually follows within a frame.

## Consent and auto mode

`SETUP.md`'s first message asks one plain question, in the person's chat, before anything is downloaded: this downloads Ben's program, checks its signature, runs it, and the page lets the person pick what gets installed. The trial showed a chat "yes" is what auto mode needs. Each install is then the person's own pick on the list screen. Whether one up-front yes covers a server that installs things is untested, and it's the first thing the next trial checks.

## Code

- **A new core package, `internal/flow`:** steps as data, each with `check`, `act` and the screens it shows, over a `Runner` interface for every outside command, so tests use a fake. Reuses `detect`, `screen`, `session` and the `clt`/`brew` watchers.
- **Runs inside the server,** which writes every step to the session log, as `show` does today. The commands stay stateless and read the log.
- **The CLI is thin:** `run`, `wait` and `resume` are adapters over `flow` and `session`.
- **Every handoff has a test** that drives the binary and reads the remedy, as the CLI rules ask.

## What changes elsewhere

- **`SETUP.md` (`W2yASt`) shrinks** to: consent, fetch and verify, `run`, loop on `wait`, handle handoffs, and the chat-only fallback. The fallback is now a second copy of the flow, written in prose. Both read `catalog/tools.json`, but the order and checks can drift. A test that compares the fallback's steps with `flow`'s list would catch that.
- **`6jPdjy`'s screens are unchanged.** The server shows them instead of the agent.
- **New work, under `VNF9sG`:** the `flow` core, the installers behind the `Runner`, `run`/`wait`/`resume`, and the next release and trial.

## Risks

- **The binary now changes the Mac.** A bug in `flow` can strand someone partway. Mitigations: every step checks before it acts, every unexpected result hands off, and the log holds each command and its output.
- **Homebrew's `.pkg` URL and version move.** The server finds the latest release, so it needs the network, and it must check the signer and notarization rather than a pinned checksum.
- **More trust is asked up front.** The consent wording has to say plainly that the program installs what the person picks.

## Settled since

1. **The reversal is approved** (Ben, 2026-11-05).
2. **The folder step moves from first to the start of the project part** (Ben asked whether it must come first; it needn't). The installs are machine-wide, and only the project steps care where they run. The server asks for a name on the existing `naming` screen and makes `~/Projects/<name>`, unless the starting folder is already a project, a Git repo, in which case it sets up in place. It runs every project command in that folder, so the agent never restarts mid-setup. The person's next session starts there: open a new Terminal window, which the [VM runs](2026-11-05-vm-run-findings.md) already require so `brew` is found by name, then `cd` and start the agent. Nothing in the starting folder moves.
3. **GitHub moves into the software** (Ben: "Could we move the Github onboarding to the software?"). Only making the account needs the person in a browser, so the page opens `github.com/signup`, with "I've made my account" and "I already have one". That's an honest button, because nothing on the Mac can see an account. The server then runs `gh auth login --hostname github.com --web --git-protocol https --skip-ssh-key`, shows the one-time code on the page with a button to `github.com/login/device`, waits for `gh` to exit, and confirms with `gh auth status`. **Untested:** `gh` run by a background process with no terminal. Check it in the VM before building.
4. **The program never asks for the person's admin password** (Ben, 2026-11-05: "I don't think we should take their admin password"). Apple's tools and Homebrew install through their own windows, which the person clicks through, even though one password dialog for both would mean fewer steps. The spike showed both windows open and come to the front from the detached server ([VM runs](2026-11-05-vm-run-findings.md), "Fourth run").
5. **The setup never makes a commit for the person** (Ben, 2026-11-05: "I don't think we should commit for them"). The flow core's commit step (`StepCommit` in `internal/flow`) comes out.
6. **The setup sets Git's name and email for the person, and they can skip it** (Ben, 2026-11-05: "Yes, we can run it for them. We should make it skippable"). It runs the same two commands GitHub's "Set up Git" guide gives (`git config --global user.name` and `user.email`); `gh` doesn't set these. Git refuses to commit without `user.name` and `user.email`, and the agent will commit in the person's first session. If Git already has both, nothing is asked. After a GitHub sign-in, the page offers the GitHub name and its private noreply address (from `gh api user`) in one click. Without GitHub, the page asks for a name and an email, and says the email ends up in the project's history. Alongside it: `init.defaultBranch main`, and `gh auth setup-git` after a sign-in.
7. **The `.gitignore` is built from what's known, without a question** (Ben, 2026-11-05: "If they have `job` and our other utilities installed, we can probably make the .gitignore better. Same with if they make it a web or Swift project."). It gets `.DS_Store`; `local/` and `job`'s lines, from their own steps; and, by the `making` answer, `node_modules/`, `.env` and `dist/` for a website, or `.build/`, `.swiftpm/`, `DerivedData/` and `xcuserdata/` for a Mac or iPhone app.
8. **Ben reviews the new screens' words in context,** as rendered screens, before anything ships ("I'd need to see it in context"). Today that covers `git-ask` and `settings`, which hold `TODO-WORDS`.

   > Done, 2026-11-05: Ben approved the words of `git-ask`, `github`, `github-signup`, `github-code`, `identity`, `identity-name`, `identity-email` and `settings` from the live preview ("The rest all look good"). None is a draft now; the approved words are listed in `recordedCopyChanges` in `internal/screen/screen_test.go`.
9. **Buttons sit as on a Mac: the main one on the far right, the way out on the far left** (Ben, 2026-11-05: "Normally the 'Yes' button goes all the way to the right, and the 'Cancel' or 'Skip' button goes all the way to the left"). Each screen lists its main answer first, and `screen.Describe` shows it last, so the rule lives in one place. A link button can name the choice it sits in front of: `github-signup` reads "Not now · Open the sign-up page · I've made my account".
10. **Answers of equal weight keep their order and have no main button** (Ben, 2026-11-05, for `live-yet`: "maybe no main button"). That covers `live-yet` and `making` (the `asListed` flag in `internal/screen/defs.go`).

    > **Superseded 2026-11-06:** Ben dropped `live-yet`, the live-yet question, since nothing used its answer. A new project gets `stage-build`, a project set up in place gets no stage, and `new` has no `--live` flag. This now covers only `making`.
11. **A screen too tall for the space under the object sits low**, over its lower part, and the object never changes size (Ben, 2026-11-05). `github-code` and `identity` joined `naming`, `identity-name` and `identity-email`.

> **Settled 2026-11-06 (Ben), from [the clean-Mac trial](2026-11-06-clean-mac-trial-findings.md):** the naming screen gets a "just the tools" choice. It skips the project steps (folder, Git, rules, what they're making) and keeps the machine-wide ones (GitHub sign-in, Git's name and email, Claude's helpers, the phone). The done handoff then says how to make a project later. The new words go to Ben as rendered screens. Leaf `VGCn9C`.
