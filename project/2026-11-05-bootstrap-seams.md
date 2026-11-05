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

A frame is `session.Frame`: `{"seq", "lost", "answered", "ended", "screen"}`.

- **`screen`** is a `screen.Description`, or `null` before the agent's first `show`. The page should draw only the object until one comes.
- **`seq`** goes back in the answer, so a click can't land on the wrong screen.
- **`lost`** arrives with `screen` already set to the lost screen, so the page draws it like any other.
- **`ended`** is a real stop. The stream closing without it means the server went away, and that's when the page shows the lost screen on its own.

Answer replies are `{"saved": true, "lost": false}`, or `{"error": "…"}` with 403 (refused), 409 (stale or already answered), 415, 422 (a choice or name it won't take) or 400.

**The contract for JavaScript** is `internal/screen/samples/<screen>.<agent>.json`, also embedded as `screen.Samples` for the preview route. `go test ./internal/screen` compares them with the Go definitions, and `-update` regenerates them. Words reach the page as spans (`{"text", "url"}`), never HTML, so the page has nothing to inject. Choices carry a stable `id`, which is what `wait answer` returns.

## What an agent gets

Every command prints rows, ending in `next:`, and `--json` gives a stable shape. Errors go to stderr, or to stdout as JSON with `--json`, and always carry `next`.

The session lives in `~/.bootstrap/`, private to the user: `session.jsonl`, `server.json` (holds the token) and `server.lock`. Pro mode's preferences can sit beside them. A stopped session's log is kept as `session-<id>.jsonl` when the next `serve` starts. Whether a server is running comes from the lock, which the kernel drops when the process dies, so nothing has to trust a stale pid.

The default `wait` timeout and the allowance are constants in `internal/session/session.go`. The lost screen's words are constants in `internal/screen/defs.go`.

`BOOTSTRAP_DIR`, `BOOTSTRAP_ROOT`, `BOOTSTRAP_OPEN` and `BOOTSTRAP_PORT` are seams for the tests, and aren't documented to users.

## Open problems found while building

- **Nothing tells the agent when to leave a "your turn" screen.** The prototype's "I clicked it" and "I did it" buttons moved `apple-ask` on to `apple-wait`, and `terminal` on to `brew-wait`. With them gone, `wait clt` and `wait brew` only return once the install has finished, which is too late for an "Adding…" screen. Both screens are built, and an agent can show one after `wait` exits 3, but that's a guess about what the person has done. `SETUP.md` or the trial has to settle this.
- **A sandboxed agent can't reach `~/.bootstrap` either.** Inside Claude Code's sandbox, `serve` exits 5 at `mkdir`, before it ever tries to bind, and that reads as the normal fallback (run on this Mac, 2026-11-05, with `bootstrap serve --agent claude-code --json`). But an agent that runs only `serve` outside the sandbox would then fail on `show`, because every command writes to that folder.

## Decisions the plan left open

- **The folder is `~/.bootstrap`**, because the house rules say `~/.tool/`.
- **The hour counts from when the agent was lost, not from the server's start.** The plan puts "at most about an hour" in the sentence about the time after losing the session. Counting from the start would end a healthy run that spends an hour on Apple's tools and Xcode.
- **"Your turn" screens can show the lost screen.** `apple-ask` and `terminal` wait on a person, but have nothing on the page to click, so "nothing changes until they click" would mean forever. The agent should be running `wait clt` or `wait brew`, and each one renews the allowance, so going past it really does mean the agent has gone.
- **A renewal never shortens an expectation.** After `show --expect 20m`, a `status` doesn't cut the 20 minutes to 3. A new `show` replaces it.
- **`show` to a closed page saves the screen and exits 5**, naming `serve`. The next `serve` draws it. Exit 0 would say someone saw it.
- **`trouble` takes `--reason` and a tool from the whole catalog.** Reasons are `network`, from the prototype, and `other`, because the network line would be wrong for any other failure. It only offers to skip a tool that can be skipped.
- **`show tool` refuses a tool the person skipped**, so an agent can't install something they said no to.
- **A typed name allows letters, numbers, dots, dashes and underscores**, starting with a letter or number. Spaces and non-ASCII letters are refused, because agents don't reliably quote.
- **A sleeping machine doesn't use up the allowance or the tab grace.** The server spots a gap of over 30 seconds between readings, which it reads every 200ms.
- **A tab gets 30 seconds to come back**, for a reload, and the first tab gets 10 minutes, in case someone has to open the link by hand.
