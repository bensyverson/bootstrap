# The `bootstrap` binary plan

**Status: approved by Ben, 2026-11-05, and imported under `VNF9sG`; the tracker is the current version of the tasks.** This details that job from [the setup link plan](2026-11-05-setup-link-plan.md), which stays the record of the overall goal. Ben ruled that the agent drives the flow and that the page is only a viewer and a channel for decisions, with everything happening in the main session. He left the verbs to the agent, under the house CLI rules (`agents-md` `modules/cli-design.md`).

The goal: a program a new Mac can run with nothing else installed, which shows the approved page, tells the agent what the user chose, and never lets the agent invent the wording. The agent still does every install.

## How it works, in one run

The agent runs these commands. Each prints terse rows by default, and a stable shape with `--json`.

| Command | Class | What it does |
|---|---|---|
| `bootstrap` | Help | Prints what this is, the verbs by role, and the one command to run first. Needs nothing set up. |
| `bootstrap detect` | Read | Reports the chip, macOS version, folder, and what's already installed. |
| `bootstrap tools` | Read | Lists what each tool is, why it's there and whether it can be skipped. |
| `bootstrap screens` | Read | Lists every screen, with the statuses and reasons each one takes. |
| `bootstrap status` | Read | Reports the session: the current screen and every answer so far. A new session starts here. |
| `bootstrap serve --agent claude-code` | Act | Starts the page in the background, opens the browser and returns. |
| `bootstrap show <screen> [<tool>] [--status y] [--expect 20m]` | Act | Moves the page to a screen. |
| `bootstrap wait <answer\|clt\|brew> [--timeout 90s]` | Act | Returns the answer or the arrival. Running out of time is exit 3, "still waiting, run wait again". |
| `bootstrap stop` | Act | Ends the session. |

A typical stretch: `show list`, `wait answer`, then `show tool jobs --status installing`, `brew install …` by the agent, and on to the next tool.

> **Addition, 2026-11-05.** `show` also takes `--reason`, for the trouble screen. What the core and server leaves settled is in [the seams doc](2026-11-05-bootstrap-seams.md).

## What the house CLI rules changed

The first draft broke several of the rules in `modules/cli-design.md`, so this version differs from it:

- **Output:** terse rows by default with `--json` on request, instead of JSON always.
- **Running out of time:** `wait` exits 3 with a reason, instead of exit 0 with "still waiting".
- **Exit codes:** the house table. A malformed command or unknown screen is 2, and no session or a failed bind is 5.
- **Grammar:** `wait clt` and `show tool jobs` follow `<verb> <target>`, replacing `--for` and `--tool`.
- **Teaching:** a bare `bootstrap` is a primer, every error names the next command, and an ack says what to do next. `show list` answers with `next: bootstrap wait answer`.
- **State:** a log file instead of a CLI that talks to a server. See below.
- **`--agent` stays, not `--as`.** It picks the app name people read, rather than attributing a write. It defaults from the environment where a harness sets something usable, and the flag wins.
- **Tests drive the binary**, asserting output bytes and exit codes, and every error path has a test that reads its remedy.

## Decisions

**The agent drives, and the binary only displays.** The agent picks a screen by name and the binary fills in the words. The alternative was putting the flow inside the binary and having it tell the agent what to do next. That would move the judgment out of the agent and duplicate `SETUP.md`, and [the plan](2026-11-05-setup-link-plan.md) says judgment stays with the agent.

**Screens and statuses are closed lists.** An unknown screen, tool or status exits with code 2 and lists the valid ones, so an agent can't invent wording even by accident. The trouble screen takes a reason from a list too, such as `network`.

**The binary works out progress.** The agent never passes a percentage. Position comes from the screen, the tool and what the user chose, so the object's color can't drift from the truth.

**The page knows nothing about any particular screen.** The server sends a description (title, status, choices, rows, code, field, card, working or waiting, progress) and the page draws it. All copy lives in one Go package, including the `{app}` and `{ai}` swap. JavaScript can't import a Go struct, so a checked-in sample of each screen's JSON is the contract: a Go test generates and compares them, and the page's preview route draws from the same samples.

**One file holds the facts about each tool.** `catalog/tools.json` is embedded in the binary and also readable as a raw file in the repo. The plan asks for both "all copy lives in the binary" and "the chat fallback has the same facts", and only a shared file satisfies both.

**The session is an append-only log, and the commands are stateless.** The house rules prefer stateless commands over a stateful file to a daemon. So `show` appends to a private JSONL file, the page's answers land in the same file, and `wait` and `status` read it. The CLI never talks to the server, so it needs no port or token.

The server exists only because a browser page needs live updates from a local address, which is the trigger the rules ask to have written down. It tails the log, sends new screens to the page, and appends the answers. The log also gives a new session everything it needs to pick up, through `status`.

**`wait answer` only returns answers to the current screen.** An answer that arrived before `wait` ran is kept and returned. An answer to an earlier screen is never returned, so an agent can't act on a stale click.

**`serve` detaches itself.** It re-launches itself in the background and returns. This doesn't rely on the harness's background feature, which OpenCode may not have in the same form. It's also why the server needs a way to notice the agent has gone, because a detached server outlives the session. A second `serve` while one is running reuses it. Only the page needs the token, and it's the only client of the server.

**Losing the agent is recoverable, and the page says so.** Every command renews a time allowance: 3 minutes by default, or longer with `--expect`. There are two cases:

- **While the agent is working:** past the allowance, the page shows the lost screen.
- **While the page is waiting for the user:** a person can read for as long as they like, so nothing changes until they click. The click is saved in the log, and if no agent has checked recently, the page shows the lost screen.

The title is "Whoops! We lost your {app} session.", and the line under it is "Open it back up and type “continue”." (Ben's wording, 2026-11-05.) This replaces the prototype's "{app} stopped."

A false alarm has to heal itself, so the next `show` or `status` brings the page straight back. The server therefore keeps running after the session is lost, for as long as the tab is open and at most about an hour. If the tab is closed or the hour is up, it exits, and `serve` starts a fresh one from the same log.

> **Correction, 2026-11-05:** "at most about an hour" no longer holds on the `chat` screen. The agent runs no commands while it waits for a reply in the chat, so that screen never shows the lost screen, and the server stays up for as long as the tab is open. See [the seams doc](2026-11-05-bootstrap-seams.md).

**`wait clt` and `wait brew` replace the "I did it" buttons.** The binary watches for `/Library/Developer/CommandLineTools/usr/bin/git` and `/opt/homebrew/bin/brew`. The plan rules out those prototype buttons, and agents can't poll by sleeping, since Claude Code blocks a foreground `sleep`. The `brew` file may appear before Homebrew's installer finishes, so the VM run has to check that.

**`detect` says what kind of folder this is, and the agent decides what to do (Ben, 2026-11-05).** People often start in a folder that isn't a project: the home folder, `~/Desktop`, `~/Documents`, `~/Downloads`, or one with a generic name like "Claude" or "Claude Code". `detect` reports the kind, and whether the folder holds files or a Git repo. The agent then offers to make a project folder and restart the session there, without moving anything that's already in the folder. A Git repo wins over a generic name, so a real project that happens to be called "Claude" isn't flagged. The list of generic names lives with the catalog, so adding one needs no code change.

**New project folders default to `~/Projects`, and `~/git` is the other suggestion (Ben, 2026-11-05).** `~/Desktop` and `~/Documents` may sync to iCloud, which is known to be hard on Git repos, and macOS asks permission before apps read those folders. Neither behavior has been tested here, but both make those folders poor defaults.

**Settled for the cards and Apple's step (Ben, 2026-11-05).** The Woodcase card shows three real renders from `woodcase-app.pen`, and the Jobs card keeps its screenshot, because it comes from a public repo. Apple's step stays separate, because it could be lengthy.

**The log is private.** It sits in bootstrap's own folder, readable only by the user. The server treats a typed project name as untrusted and accepts only a safe folder name, because an agent will use it in commands.

**Go standard library only, with cgo off.** So there are no new Go dependencies, and the binary needs no developer tools to run. Three.js r128 is vendored with its license, as the plan already requires.

**Apple silicon only, and the "needs a newer Mac" screen goes.** An arm64 binary can't start on an Intel Mac, so that screen could never appear. `SETUP.md` checks `uname -m` before downloading and says it in the chat. A universal binary would keep the screen, but it adds size and build steps to show one sentence that the chat can say just as well.

**A separate release path from the tap.** The tap's `scripts/release` publishes formulae, and its bottle lives inside a Homebrew archive, while this needs a bare signed file on a GitHub release that `curl` can fetch. So this repo gets its own small release script, using the same signing identity. Tap distribution is a later addition.

**Verifying the download uses only stock tools.** `shasum` and `/usr/bin/codesign` both exist in stock macOS, and `otool -L` shows `codesign` doesn't link `xcselect`, so it shouldn't need the developer tools. That was read on this Mac, which has Xcode, so the VM has to confirm it. As far as I know, a bare binary can't have its notarization stapled, so `spctl` would need a network check. The release leaf checks this.

## Order

1. **Core** and **server** come first, because everything else depends on those commands.
2. **The page** runs alongside the server once the screen description exists.
3. **The trial release** includes only what the agent trial (`Y6LsGy`) needs, because that trial could change the design. It ships the welcome and list screens, with the rest of the screens following once the trial has answered.
4. **The full screens** take the trial's findings, Ben's rulings and real card images.

The core and server share one module and most of their context, so one agent could take them in sequence. The page touches different files and can run in parallel.

## Still open

- **What "open it back up and type continue" really does.** A session reopened with its history understands "continue". A brand-new session in that folder doesn't, and on a new Mac there may be no `AGENTS.md` yet to tell it. This has to be tried in both apps, and it may change the line. If so, the fallback is to paste the same link, which leads to `status`.
- **How long `wait` should block.** Claude Code's foreground limit is 2 minutes by default, and a background call can notify the agent when it ends. OpenCode's limits are unchecked. The trial should show whether the loop is calm, and what an agent does when someone walks away for an hour.
- **Getting stuck in Terminal.** With no "I did it" button, the page has no signal that Homebrew's install failed. Past some time, `wait brew` exits 3 and the agent should ask in the chat. The VM run sets the length.

  > **Correction, 2026-11-05:** this assumed people paste Homebrew's command into Terminal, and Apple now blocks that with a malware warning. The `terminal` screen is due to be replaced by Homebrew's `.pkg`. See [the VM run](2026-11-05-vm-run-findings.md).


## Tasks

Imported into `job` on 2026-11-05 under `VNF9sG`; the tracker is the current version.

```yaml
tasks:
  - title: Build the bootstrap core
    ref: core
    desc: |
      The `bootstrap` binary needs a library that holds its decisions, with the CLI and server as thin layers over it. This leaf makes that library. Everything else in the binary builds on it.

      What exists when it's done: a Go module at the repo root using only the standard library, with cgo off. `bootstrap`, `detect`, `tools` and `screens` work, printing terse rows by default and a stable shape with `--json`, and none of them needs a session. The catalog of tools lives in `catalog/tools.json`, embedded in the binary and readable as a raw file, so the chat fallback in `SETUP.md` can use the same facts when the binary can't run. The screens from the prototype are defined in Go as closed lists, each producing the description the page draws, including the swap of `{app}` and `{ai}` for Claude Code or OpenCode. Progress is calculated from the screen, the tool and the user's choices; callers never pass a percentage.

      Already decided (project/2026-11-05-setup-link-plan.md, "The interface", and project/2026-11-05-bootstrap-binary-plan.md): Go; all copy in the binary; very few words at a 6th-grade reading level; the tool is called Jobs in anything a person reads. Detect Apple's tools by `/Library/Developer/CommandLineTools/usr/bin/git`, never `xcode-select -p` or `command -v git`, because `/usr/bin/git` is a stub until they're installed. Copy each screen's words from the `SCREENS` object in project/2026-11-05-setup-page-mock/prototype/template.html.

      Seams: the leaf "Build the server and CLI" consumes this; the leaf "Port the page" draws the description. Check in one JSON sample per screen, generated and compared by a Go test, because JavaScript can't import a Go struct and the page is the other side of that seam.

      Out of scope: the server, the page and signing. The "needs a newer Mac" screen is dropped, since an arm64 binary can't start on an Intel Mac. The prototype's study bar and fake waits aren't part of the product.

      The house CLI rules bind this leaf and the server leaf: read project/agents/cli-design.md. They cover the primer, one shape per verb, the shared exit codes, errors that name the next command, and tests that drive the binary.

      The agent's call: exact field names and package layout, keeping files small.
    criteria:
      - A table test shows detect reports each missing and present state, using a fake filesystem root, including Apple's tools absent while /usr/bin/git exists
      - A test driving the binary shows an unknown screen, tool or status exits 2 and lists the valid values
      - A test shows a bare bootstrap prints the primer with no session and no network
      - A test shows every screen's description matches its checked-in JSON sample, for both Claude Code and OpenCode
      - A test shows progress never goes down across the screen order, with tools skipped
      - catalog/tools.json loads from the embedded copy, and a test shows every tool on the list screen is in it
  - title: Build the server and CLI
    ref: server
    blockedBy: [core]
    desc: |
      This is how the agent and the page talk. An agent runs short commands and a person clicks in a browser, and this leaf connects the two without opening anything to the rest of the machine.

      What exists when it's done: `bootstrap serve --agent <name>` detaches itself, binds to 127.0.0.1 on a free port, opens the browser with `/usr/bin/open`, and reports the URL and whether the browser opened. The session is an append-only JSONL log readable only by its owner. `show` appends to it, the page's answers land in it, `wait` and `status` read it, and `stop` ends the session. The commands never talk to the server. The server tails the log, sends new screens to the page over Server-Sent Events, and appends the answers it receives. `wait answer|clt|brew` returns what it waited for, or exits 3 with "still waiting, run wait again". Answers to an earlier screen are never returned.

      Already decided (project/2026-11-05-setup-link-plan.md): bind to loopback, require a random token on the page's requests, check Host and Origin, exit after the agent is gone. Standard library only. From project/2026-11-05-bootstrap-binary-plan.md: every command renews a time allowance, three minutes by default or longer with `show --expect`. Past it while the agent is working, the page shows the lost screen, "Whoops! We lost your {app} session. Open it back up and type “continue”." (Ben's wording). While the page waits on the user nothing changes until they click, when the click is saved and the lost screen appears if no agent has checked recently. The next command from any session brings the page back. The server keeps running while the tab is open, up to about an hour, then exits, and `serve` starts fresh from the same log. A second `serve` reuses a live session. A typed project name must be a safe folder name, because an agent will use it in commands. The house CLI rules bind this leaf: project/agents/cli-design.md.

      Seams: the library from "Build the bootstrap core" holds the decisions; this layer stays thin. The page is another leaf, so test with plain HTTP clients.

      Open, the agent's call: where bootstrap's folder lives, which pro mode's preferences file (t5FAqu) will share. Open for the trial, so make them easy to change: the default `wait` timeout, and the exact lost-screen line.

      Risks: `serve` must fail with clear JSON rather than hang when it can't bind or open a browser, because that failure is what sends the agent to the chat fallback. A sandboxed agent can't bind a port or run `open`, and that should read as a normal fallback. A machine sleeping through an allowance shouldn't count as the agent leaving, or at least should recover when it wakes.
    criteria:
      - Tests show a request is refused for each of a missing token, a wrong token, a wrong Host and a wrong Origin
      - A test shows the server binds only to the loopback address
      - Tests show the lost screen appears past the allowance while working, and only after a click while waiting on the user
      - A test shows the next command after a lost screen brings the page back
      - A test shows a click with no agent is saved and a later status reports it
      - A test shows the server exits once the tab is closed or the hour is up
      - Tests driving the binary show wait returns an answer and a clt or brew arrival at exit 0, and exit 3 with the remedy when time runs out
      - A test shows wait answer never returns an answer to an earlier screen
      - A test shows serve exits 5 with a remedy when it can't bind, and doesn't hang
      - A test shows the log is readable only by its owner, and that an unsafe project name is refused
      - A live run of serve, show, wait and stop leaves no process behind, shown by pgrep on the printed pid
  - title: Port the page from the prototype
    ref: page
    blockedBy: [core]
    desc: |
      This is what a new user actually sees, and Ben has approved its look and motion. The job is to carry that across faithfully while removing everything that only served the study.

      What exists when it's done: a page embedded in the binary that draws any screen from the description the server sends. It holds the threads, the transitions, the fonts and the cards, and knows nothing about any particular screen's wording. A preview route draws every screen in each state from the checked-in JSON samples, for review and for still tests.

      Already decided (Ben, 2026-11-05; project/2026-11-05-setup-link-plan.md, "The interface"): silk threads in linear light, blue through cyan to green with progress, relaxing to the starting form. No hard cuts: what leaves and what arrives share one 1.4s S-curve, and the object never moves. Fast while working, slow while waiting. Grand Medium for titles and Regular for buttons, subset to Latin. Three.js r128, the fonts and the images are embedded, with no CDN. Copy the shaders and timing constants from project/2026-11-05-setup-page-mock/prototype/template.html rather than rewriting them.

      Keep the prototype's reduced-motion handling, and its eight-bit path for when the float buffer isn't available. Keep Three.js's license with the vendored copy. Tool names link to their repos.

      Out of scope: the bottom study bar, fake waits and "I did it" buttons. Real card contents for Homebrew, Git and Woodcase belong to the full-screens leaf.

      Risks: headless Sleepy never ticks animation frames, so a page that fades in screenshots as blank. Test stills through the preview route at rest. Live motion can only be judged in a real browser, so Ben reviews it. The server sends no third-party requests, so a Content-Security-Policy can prove the page loads nothing from outside.
    criteria:
      - Sleepy stills of every screen through the preview route, compared against the prototype with peep, show no unexplained differences
      - The page loads with the network off and a strict Content-Security-Policy, with no console errors, shown by sleepy
      - No overflow at 400px wide on any screen, shown by sleepy
      - The page shows the lost screen when the server goes away, with no request needed, shown by a live run
      - Ben has reviewed the live motion in a real browser and approved it, recorded in a note
  - title: Publish a signed trial release
    ref: release
    blockedBy: [server, page]
    labels: [decision]
    desc: |
      The agent trial (Y6LsGy) asks whether an agent, given only a pasted link, will verify and run a signed binary. That needs a real signed file at a public URL. It could change the design, so it comes before the rest of the screens.

      What exists when it's done: `scripts/release` in this repo builds for darwin/arm64 with cgo off, signs with the Developer ID identity (the same one as project woodcase's scripts/release.conf), notarizes, and uploads a bare file with its checksum to a GitHub release of the public bensyverson/bootstrap. A thin README and a stub SETUP.md say enough for the trial: what this is, what will change, how to fetch, verify and start it. The welcome and list screens work end to end.

      Already decided: public repo bensyverson/bootstrap, approved by Ben. Fetched with curl since Homebrew may not exist yet; SETUP.md pins a version and checksum and the agent verifies those and the signature before running it (project/2026-11-05-setup-link-plan.md).

      Needs Ben: his OK on timing, and his signing and notary credentials, which a sandboxed agent can't reach. Pushing a release is outward-facing, so ask at the moment of doing it.

      Open, to settle here: the tap's `scripts/release` publishes formulae and bottles rather than a bare file, so this needs its own small script, and Ben has yet to rule on when it goes public. Confirm which notary profile applies. Confirm that `codesign` and `spctl` run with no developer tools, and whether a bare binary can be stapled; as far as we know it can't, so `spctl` may need a network check. Set the oldest macOS the binary targets and state it.

      Out of scope: the full README and SETUP.md (W2yASt), and the tap formula, which can come later.
    criteria:
      - A file fetched with curl from the public release passes the checksum, codesign and spctl checks the stub SETUP.md lists, shown by a recorded run
      - The release URL and raw SETUP.md load with no sign-in, shown by curl with no credentials
      - Ben has approved the timing, recorded in a note
      - A request without the token is refused by the released binary, shown by curl
  - title: Finish every screen and the real cards
    ref: screens
    blockedBy: [release]
    desc: |
      The trial release only needs two screens. This leaf finishes the rest, after the trial has shown whether the design holds, so it isn't built on a premise that's about to move.

      What exists when it's done: every prototype screen works through `show`, in Claude Code's and OpenCode's names: Apple's tools, Homebrew, each tool, the project questions, GitHub, the phone, done, trouble and lost. Cards show real captures instead of the invented ones. The Homebrew, Git, Contextfill and Woodcase cards in the prototype are made up or illustrative.

      Already decided: see project/2026-11-05-setup-link-plan.md, "The interface". Long steps may only be able to show elapsed time, and the Command Line Tools step shows no estimate, because no reliable figure exists (project/2026-11-05-clean-mac-research.md).

      Open, Ben's call: whether Apple's step stays separate, since Homebrew can install those tools itself; which file goes on the Woodcase card (he suggested a real screenshot from `woodcase serve`, and it has to be legible at card size); whether the Jobs card keeps its real task titles. Apply whatever Y6LsGy finds, editing this description if it changes the premise.

      Out of scope: pro mode (t5FAqu), which builds on this, and the /remote-control step's content (9WWQm2).
    criteria:
      - Sleepy stills of every screen in both agent names are reviewed by Ben, recorded in a note
      - A run with the screens driven by a script reaches done with each state's choices returned to wait as JSON
      - No card shows invented output, shown by each card's source being listed in a note
      - On a Mac with no developer tools, the page opens and the agent receives the choices, shown on the clean-Mac run or the VM
```
