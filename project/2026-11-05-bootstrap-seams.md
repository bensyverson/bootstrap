# The `bootstrap` binary's seams

What the core and server leaves (`rgABs5`, `Bm1yQO`) settled, for whoever builds on them. [The binary plan](2026-11-05-bootstrap-binary-plan.md) holds the reasoning. Code is the authority on exact fields; this says where to look and why.

## Layout

| Path | Holds |
|---|---|
| `catalog/tools.json`, `catalog/generic-folders.json` | The facts. Embedded, and readable raw by the chat fallback. Adding a generic folder name needs no code change. |
| `internal/detect` | What's on this Mac, and the folder's kind. Only reports. |
| `internal/screen` | Every word, the closed lists, progress, and the description the page draws. |
| `internal/session` | The log, replaying it, who's lost, and `wait`. |
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
- **`ended`** is a real stop. The stream closing without it means the server went away, and that's when the page shows the lost screen on its own.
- **`gone`** is the lost screen's description, on every frame until `ended`, so those words are already in the page when there's no server left to ask. The page waits 2.5 seconds (`LOST_AFTER_MS` in `live.js`) in case the connection heals, as it does after a Mac wakes, and keeps retrying underneath, so a server that comes back redraws the real screen.
- **`answered`** dims the choices and speeds the object up, since it's the agent's turn. This is the page's only state the prototype didn't have, because its buttons moved on at once.

Answer replies are `{"saved": true, "lost": false}`, or `{"error": "…"}` with 403 (refused), 409 (stale or already answered), 415, 422 (a choice or name it won't take) or 400.

**The contract for JavaScript** is `internal/screen/samples/<screen>.<agent>.json`, also embedded as `screen.Samples` for the preview route. `go test ./internal/screen` compares them with the Go definitions, and `-update` regenerates them. Words reach the page as spans (`{"text", "url"}`), never HTML, so the page has nothing to inject. Choices carry a stable `id`, which is what `wait answer` returns.

## What an agent gets

Every command prints rows, ending in `next:`, and `--json` gives a stable shape. Errors go to stderr, or to stdout as JSON with `--json`, and always carry `next`.

The session lives in `~/.bootstrap/`, private to the user: `session.jsonl`, `server.json` (holds the token) and `server.lock`. Pro mode's preferences can sit beside them. A stopped session's log is kept as `session-<id>.jsonl` when the next `serve` starts. Whether a server is running comes from the lock, which the kernel drops when the process dies, so nothing has to trust a stale pid.

The default `wait` timeout and the allowance are constants in `internal/session/session.go`. The lost screen's words are constants in `internal/screen/defs.go`.

`BOOTSTRAP_DIR`, `BOOTSTRAP_ROOT`, `BOOTSTRAP_OPEN` and `BOOTSTRAP_PORT` are seams for the tests, and aren't documented to users.

## Open problems found while building

- **Nothing told the agent when to leave a "your turn" screen.** The prototype's "I clicked it" and "I did it" buttons moved `apple-ask` on to `apple-wait`, and `wait clt` and `wait brew` only returned once the install had finished. Ben ruled on 2026-11-05 to stay on the "your turn" screen until the install finishes, and to keep `apple-wait` and `brew-wait` only if the VM run found a signal.

  > **Settled, 2026-11-05, by [the VM runs](2026-11-05-vm-run-findings.md):** Apple's step has one. `wait clt` now returns `started` when a folder holding `CLTools_` files appears in `/Library/Updates` during the wait, which happens when the person clicks Agree; the agent then shows `apple-wait` and waits again. It returns `arrived` on Git's file, and `canceled` (exit 4) when Apple's installer was running during the wait and quit without Git. Only folders that appear during the wait count, so a second wait, or a folder left by an earlier try, doesn't fire. Homebrew's `.pkg` has no start signal yet, so `wait brew` returns only `arrived` or `canceled`, and nothing leads to `brew-wait`. The process names and folder live in `catalog/tools.json`, so the chat fallback has them too.

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
