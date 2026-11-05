# The `bootstrap` binary's seams

What the core and server leaves (`rgABs5`, `Bm1yQO`) settled, for whoever builds on them. [The binary plan](2026-11-05-bootstrap-binary-plan.md) holds the reasoning. Code is the authority on exact fields; this says where to look and why.

## Layout

| Path | Holds |
|---|---|
| `catalog/tools.json`, `catalog/generic-folders.json` | The facts. Embedded, and readable raw by the chat fallback. Adding a generic folder name needs no code change. |
| `internal/detect` | What's on this Mac, and the folder's kind. Only reports. |
| `internal/screen` | Every word, the closed lists, progress, and the description the page draws. |
| `internal/session` | The log, replaying it, who's lost, and `wait`. |
| `internal/flow` | The setup as data, run by the server (below). `flowtest` is a pretend Mac for its tests. |
| `internal/server` | Loopback HTTP and Server-Sent Events, the refusals, and how a server starts and exits. |
| `internal/page` | The one embed seam. The real page replaces `internal/page/files/`, and the server doesn't change. |
| `internal/cli`, `cmd/bootstrap` | Parsing, rows and exit codes. |

## What the page gets

The server has three routes, and every request needs `?token=` from the printed URL, the right `Host`, and, when it sends `Origin`, the right one. `POST` must send `Origin`.

- **`GET /`** and **`GET /<file>`** serve `internal/page/files/`, with the token on those requests too. A page that loads other files adds `location.search` to each URL, or inlines them, as the prototype does.
- **`GET /events`** streams one `data: <frame>` line per change, and sends the current frame on connect.
- **`POST /answer`** takes `{"seq": 7, "choice": "lets-go", "tools": ["sleepy"], "name": "my-project"}` as `application/json`.

- **`GET /preview`** serves the same page, which then draws a state from `/preview.json` (`page.Previews()`) instead of the stream. `/preview#tool.jobs.installing.claude-code` opens one state at rest for stills. A later change of address, or the arrow keys, walks the states with the real transitions. Plain `/preview` lists them. Previews never touch the session, so they don't count as a tab: leave a real tab open if a sweep will outlast the 10-minute first-tab grace.

Every response carries a Content-Security-Policy (`page.Policy`): `default-src 'none'`, and `'self'` for scripts, styles, fonts, images and connections. The token can't reach static tags, so `index.html` holds one inline loader that adds `location.search` to each file, and the policy allows it by a hash worked out from the file when the server starts. So editing the loader needs no second edit. Fonts load through `FontFace` for the same reason, and cards are `<template data-card="…">` elements in `index.html`, so swapping one touches nothing else. Nothing may use a style attribute or `innerHTML`, and `internal/page/page_test.go` checks both.

A frame is `session.Frame`: `{"seq", "lost", "answered", "ended", "screen", "gone"}`.

- **`screen`** is a `screen.Description`, or `null` before the agent's first `show`. The page should draw only the object until one comes.
- **`seq`** goes back in the answer, so a click can't land on the wrong screen.
- **`lost`** arrives with `screen` already set to the lost screen, so the page draws it like any other.
- **`ended`** is a real stop. The stream closing without it means the server went away, and that's when the page shows the lost screen on its own. A stop on any screen but `done` (or before any screen) arrives with `screen` set to the internal `end` screen, built like `lost`: "That’s all for now." and "You can close this and go back to {app}." (Ben's words, 2026-11-05). It is settled and not answered, so the object relaxes. A stop on `done` keeps `done`.
- **`gone`** is the lost screen's description, on every frame until `ended`, so those words are already in the page when there's no server left to ask. The page waits 2.5 seconds (`LOST_AFTER_MS` in `live.js`) in case the connection heals, as it does after a Mac wakes, and keeps retrying underneath, so a server that comes back redraws the real screen.
- **`answered`** puts the screen in its waiting state and speeds the object up, since it's the agent's turn. This is the page's only state the prototype didn't have, because its buttons moved on at once. The page enters it on the click itself, without waiting for the server: the choices fade out and a waiting line fades up in their place, on the shared 1.4 s S-curve, and the rows and the name field dim. Nothing around the choices moves. A refused answer reverses it. The line, "One moment…", is the page's own (`WAITING_LINE` in `page.js`) and is proposed, not yet approved by Ben. Previews `<screen>.answered.<agent>` show it at rest.

Answer replies are `{"saved": true, "lost": false}`, or `{"error": "…"}` with 403 (refused), 409 (stale or already answered), 415, 422 (a choice or name it won't take) or 400.

**The contract for JavaScript** is `internal/screen/samples/<screen>.<agent>.json`, also embedded as `screen.Samples` for the preview route. `go test ./internal/screen` compares them with the Go definitions, and `-update` regenerates them. Words reach the page as spans (`{"text", "url"}`), never HTML, so the page has nothing to inject. Choices carry a stable `id`, which is what `wait answer` returns.

## What an agent gets

Every command prints rows, ending in `next:`, and `--json` gives a stable shape. Errors go to stderr, or to stdout as JSON with `--json`, and always carry `next`.

The session lives in `~/.bootstrap/`, private to the user: `session.jsonl`, `server.json` (holds the token) and `server.lock`. Pro mode's preferences can sit beside them. A stopped session's log is kept as `session-<id>.jsonl` when the next `serve` starts. Whether a server is running comes from the lock, which the kernel drops when the process dies, so nothing has to trust a stale pid.

The default `wait` timeout and the allowance are constants in `internal/session/session.go`. The lost screen's words are constants in `internal/screen/defs.go`.

`BOOTSTRAP_DIR`, `BOOTSTRAP_ROOT`, `BOOTSTRAP_OPEN` and `BOOTSTRAP_PORT` are seams for the tests, and aren't documented to users.

## Open problems found while building

- **Nothing told the agent when to leave a "your turn" screen.** The prototype's "I clicked it" and "I did it" buttons moved `apple-ask` on to `apple-wait`, and `wait clt` and `wait brew` only returned once the install had finished. Ben ruled on 2026-11-05 to stay on the "your turn" screen until the install finishes, and to keep `apple-wait` and `brew-wait` only if the VM run found a signal.

  > **Settled, 2026-11-05, by [the VM runs](2026-11-05-vm-run-findings.md):** Apple's step has one. `wait clt` now returns `started` when a folder holding `CLTools_` files appears in `/Library/Updates` during the wait, which happens when the person clicks Agree; the agent then shows `apple-wait` and waits again. It returns `arrived` on Git's file, and `canceled` (exit 4) when Apple's installer was running during the wait and quit without Git. Only folders that appear during the wait count, so a second wait, or a folder left by an earlier try, doesn't fire. *(Corrected 2026-11-05: that wasn't enough. In [the third VM run](2026-11-05-vm-run-findings.md), Apple downloaded two versions into two folders a moment apart, and a second wait returned `started` again. A wait that begins with a folder present and the installer running now treats the install as under way and doesn't watch for a start.)* Homebrew's `.pkg` has no start signal yet, so `wait brew` returns only `arrived` or `canceled`, and `brew-wait` is gone (Ben, 2026-11-05). Its Homebrew card moves to `brew-ask` (Ben, 2026-11-05), because Homebrew has no other screen. Ben approved it live, with `chat`, on 2026-11-05. The Command Line Tools line now reads "Apple’s tools, which include Git.", because Homebrew doesn't need them. The process names and folder live in `catalog/tools.json`, so the chat fallback has them too.

- **A sandboxed agent can't reach `~/.bootstrap` either.** Inside Claude Code's sandbox, `serve` exits 5 at `mkdir`, before it ever tries to bind, and that reads as the normal fallback (run on this Mac, 2026-11-05, with `bootstrap serve --agent claude-code --json`). But an agent that runs only `serve` outside the sandbox would then fail on `show`, because every command writes to that folder.

## Decisions the plan left open

- **The folder is `~/.bootstrap`**, because the house rules say `~/.tool/`.
- **The hour counts from when the agent was lost, not from the server's start.** The plan puts "at most about an hour" in the sentence about the time after losing the session. Counting from the start would end a healthy run that spends an hour on Apple's tools and Xcode.
- **"Your turn" screens can show the lost screen.** `apple-ask` and `brew-ask` (formerly `terminal`) wait on a person, but have nothing on the page to click, so "nothing changes until they click" would mean forever. The agent should be running `wait clt` or `wait brew`, and each one renews the allowance, so going past it really does mean the agent has gone.
- **`chat` never shows the lost screen, and keeps the server alive.** The agent shows it before asking something in the chat, then runs no commands until the reply, so its silence means nothing (Ben, 2026-11-05). The server's hour doesn't apply either, because its exit would make the page draw the lost screen. This is the one exception to "at most about an hour": the server lasts as long as the tab stays open. `wait answer` refuses it, as it does any screen with no buttons.
- **A renewal never shortens an expectation.** After `show --expect 20m`, a `status` doesn't cut the 20 minutes to 3. A new `show` replaces it.
- **`show` to a closed page saves the screen and exits 5**, naming `serve`. The next `serve` draws it. Exit 0 would say someone saw it.
- **`trouble` takes `--reason` and a tool from the whole catalog.** Reasons are `network`, from the prototype, and `other`, because the network line would be wrong for any other failure. It only offers to skip a tool that can be skipped.
- **`terminal` is now `brew-ask`** (Ben, 2026-11-05), because Apple blocks a pasted Homebrew command with a malware warning. The agent downloads Homebrew's `.pkg`, checks it and opens it. The screen says "Your turn: click through the installer." and "This adds {link:homebrew}, which installs other software for you. It will ask for your Mac password."
- **`detect` reports free space** as `disk free_gb=… apple_step=enough|short|unknown`. `short` means under 20 GB (`detect.AppleStepNeeds`), because Apple's step refused at 18.7 GB on the VM. The agent decides what to say about it.
- **`show tool` refuses a tool the person skipped**, so an agent can't install something they said no to.
- **A typed name becomes git-friendly (Ben, 2026-11-05).** It's lowercased and spaces become dashes, so "Pine Bakery" is saved as `pine-bakery`. After that it may hold letters, numbers, dots, dashes and underscores, starting with a letter or number. Anything else, including non-ASCII letters, is refused rather than quietly dropped.
- **A sleeping machine doesn't use up the allowance or the tab grace.** The server spots a gap of over 30 seconds between readings, which it reads every 200ms.
- **A tab gets 30 seconds to come back**, for a reload, and the first tab gets 10 minutes, in case someone has to open the link by hand.

## The flow core (`3rqKIp`, 2026-11-05)

`internal/flow` is the library behind [the server-driven flow](2026-11-05-server-driven-flow.md). The verbs (`VBA13e`) and the installers (`UyKEjV`) plug into it.

- **Steps are data.** `flow.Steps()` lists the 13 steps in order (plan, apple, homebrew, tools, folder, git, github, stage, rules, commit, settings, phone, done), each with the screens it shows, the screens it asks on, and whether it can be skipped. A test comparing the chat fallback with the flow can read that list.
- **Every step checks, then acts.** A check says done, skipped (the person already said not now), or "act". An act ends done, skipped, needing the person (a question on the page, after which the act runs again from the top), or in trouble. So every act is safe to repeat, and a second run over a finished setup runs only inspections.
- **Answers come from the log.** A question already answered in this session isn't asked again, so a restarted server picks up where it was.
- **Every outside command goes through `flow.Runner`,** by full path, tagged `Inspect` or `Change`. Only the fake exists so far; the real runner is the installers' leaf. Each command's output is appended to `~/.bootstrap/steps/<tool or step>.log`.
- **Anything unexpected is a `trouble` handoff,** never a guess: an exit code the step doesn't expect, a timeout, a command that won't start, an installer that quits, a tool missing after a clean exit, too little disk, a `.pkg` not signed by 927JGANW46, or a file where the project folder goes. It carries the command, exit code, log path and last 20 lines, and the ways forward: always retry and talk, plus skip for a tool the person may go without or a step they may skip. The page shows `chat`, and the flow waits for `resume`. Phone and done are handoffs the flow doesn't wait for.
- **The folder step** sets up in place when the starting folder is a Git repo that isn't the home folder or one of its standard folders; otherwise it asks for a name and makes `~/Projects/<name>`, or uses that folder if it exists. It never moves anything.

**What the log gained.** `session` holds the one shared struct for both sides: `Step`, `Note`, `Handoff` (with `Evidence` and `Way`s) and the replayed `State.Driving`, `Step`, `Finished`, `Notes`, `Handoffs`, `Open` and `Resumed`. The agent's side is `Store.TakeHandoff` (it has seen the handoff) and `Store.Resume`. `session.WatchInstall` is `WaitInstall` without the beat, because a beat from the server would claim an agent that isn't there.

**Who's lost changed.** While a flow drives and nothing is handed off, the page never shows the lost screen and the hour doesn't run. A handoff the agent hasn't taken counts against the allowance even on the `chat` screen; once taken, `chat` waits on the person as before. A new server clears `Driving`.

**The server** takes `Config.Flow` (and `ChildOptions.Flow`), a `func(context.Context) error` it runs beside itself. The flow's context ends when the server exits, under the same page, token and tab rules as before; `Server.FlowErr` keeps why a flow stopped early.

**Placeholders.** `git-ask` (statuses `repo`, `ignore`, `commit`) and `settings` are new screens whose words start with `TODO-WORDS` until Ben writes them; `screen.Placeholders()` lists them. `catalog/tools.json` gained each tap tool's `formula`, since `jobs` installs as `bensyverson/tap/job`.

## The agent's verbs (`VBA13e`, 2026-11-05)

`run`, `wait` and `resume` are the agent's whole interface to the flow. A weak model can drive it by following `next:` lines alone. The transcript in `cmd/bootstrap/testdata/flow-retry.txt` comes from the real binary, the real flow and the real runner on a pretend Mac, with a trouble, a retry and a wait that runs out of budget.

- **`run --agent <app>`** starts the server with `ChildOptions.Flow` (the hidden verb's `--flow`), opens the page and returns `running url=… pid=… browser=… joined=no` and `next: bootstrap wait`. A second `run` joins: `joined=yes`, same pid, no second flow. `run` doesn't replace `serve`, because `serve`, `show` and `wait answer|clt|brew` stay for tests, the preview and an agent driving the page itself. `run` refuses a page that `serve` started (exit 5, `bootstrap stop, then bootstrap run --agent …`), since that page has no setup in it. `server.json` now says `flow`, so commands can tell the two apart without a race against the flow's first log line.
- **`wait`** with no target blocks until a handoff, or until its budget (`--timeout`, 90 seconds by default) runs out. Either way it prints the flow's work since the agent's last wait: `step <id> <status> <i> of <n>`, `note <text>`, each handoff with `what:`, its evidence (`command:`, `limit:`, `log:`, and the output's last lines as `  | …`), then one `state` row and one `next:` per way forward. A step's rows collapse to where it got to, and `handed-off` steps are left out because the handoff says more. Out of budget is exit 3 with the same rows on stderr (and `updates` in `--json`), then `next: bootstrap wait`. A blocking handoff is taken (`Store.TakeHandoff`) and is printed again on every wait until a resume. After the done handoff, `wait` returns at once with `state finished`. A closed page, a stopped session or no session is exit 5, naming `bootstrap run --agent …`, which picks up from the log.
- **The cursor is a `told` event in the log** (`Store.MarkTold`, read by `Store.Unread`), not a file of its own. It is written under the same lock as everything else, survives a restart, and the history shows when the agent was told what. It counts as the agent checking in. The library side is `session.WaitFlow`, and the CLI only renders.
- **`resume --retry|--skip`** answers the open handoff, with `next: bootstrap wait`. A way the handoff doesn't offer is exit 2, listing the valid ones and naming the first. With nothing open, it's exit 3 with `next: bootstrap wait`. Flags that take no value are `switches` on a command.
- **Phone and done** come out of `wait` too: `next: do what the phone handoff says in the chat, then bootstrap wait`, and `next: do what the done handoff says in the chat; the setup is over, so there is nothing more to run`. Nobody needs to `stop` afterward: closing the tab ends the server, as before.
- **`status`** says `next: bootstrap wait` during a run, and `bootstrap run --agent …` once that server has gone. With no session it now names `bootstrap run --agent <app>`, not `detect, then serve`, because `run` is how an agent starts now.
- **The real runner is `flow.ExecRunner`** (`internal/flow/execrunner.go`). It uses os/exec with the command's timeout and `Dir`, and captures stdout and stderr together. Each command runs in its own process group, so a timeout also stops whatever the command started. A command that won't start sets `Err`. Only the command's own limit counts as `TimedOut`, not the flow ending. Its `Root` is put in front of every command's path, from `BOOTSTRAP_ROOT` like the detect seam, so the binary tests run a pretend `brew` and never touch the Mac. The installers leaf (`UyKEjV`) extends it. `flow.DefaultHomebrewPkg` is Homebrew's `releases/latest/download/Homebrew.pkg`.
- **Not yet:** `run`'s acknowledgement doesn't say whether the person must click anything first. If the flow returns early with an error (`Server.FlowErr`), nothing in the log says so, and `wait` reports `still working` until the server exits.

## Cards and titles, after the live review (2026-11-05)

Ben approved these live, on top of the prototype:

- **Cards bob and sway while they wait.** There are two sine swings on the individual `translate` and `rotate` properties: a 7px rise over 3.4s and a 0.45° sway over 4.7s, both alternating. They stack on the tilt and the slide in `page.js`'s `transform`, so the 900ms arrival and departure are unchanged. A first try with four keyframes eased into each point, and Ben said it felt "like it's moving between positions rather than bobbing". The bob runs on every card, not only while a tool installs, because cards only appear on install screens and switching it on and off would jump. It stops under `prefers-reduced-motion`.
- **Cards take the buttons' colored shadow,** a little lighter, with a faint aura edge, in place of their own gray shadow.
- **Large titles are tighter:** `h1` letter-spacing goes from −0.022em to −0.032em. The prototype in `2026-11-05-setup-page-mock/prototype/` keeps the old value, because it's the record of what was approved first.

