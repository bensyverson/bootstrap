# Proposal: the server drives the setup (2026-11-05)

**Status: a proposal for Ben's approval.** It reverses a recorded decision in [the binary plan](2026-11-05-bootstrap-binary-plan.md), "The agent drives, and the binary only displays". If approved, that decision gets a marked correction pointing here.

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
| 0. Folder | Detects the kind, asks on the page, makes the folder | Restarts the session in the new folder, since only the person and the agent can do that |
| Apple's tools | Disk check, `xcode-select --install`, raises Apple's window, watches the `clt` signals | |
| Homebrew | Downloads the `.pkg`, checks it's signed by 927JGANW46 and notarized, opens it, watches for `brew` | |
| Tools | `brew trust`, then each chosen tool by full path, one screen and card each | |
| Git | Repo, `.gitignore`, first commit, each asked on the page | |
| `gh` and GitHub | Asks on the page | Walks the person through the account and `gh auth login` in the chat |
| Project | Asks the stage questions, runs `agents init` (and `agents migrate` when asked), `local/`, `job init --as <agent>`, `job gitignore` | |
| Machine settings | Asks, then runs each tool's own `hook install` | |
| Phone | Asks | Explains `/remote-control` |
| Done | Shows done, stops when the tab closes | Summarizes in the chat |

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

## Open, for Ben

1. Approve the reversal, and with it this split between the server and the agent.
2. Step 0: should the server make the new folder, with the agent then telling the person to restart there? Or should the agent keep the whole step?
3. GitHub: keep the account and `gh auth login` walkthrough in the chat, as above?
