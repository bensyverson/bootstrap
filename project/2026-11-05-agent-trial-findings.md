# Agent trial: what an agent does with the pasted link (2026-11-05)

This answers the first question in `Y6LsGy`: given only the link, will an agent verify and run the signed binary? It ran against release `v0.1.0` and the trial `SETUP.md` (commit `8bfafad`).

## Claude Code, in the VM `clt3`

**Setup.** The Tart VM `clt3` on the Mac Mini, macOS 26.6.2, arm64, with Gatekeeper on. It already had Apple's tools and Homebrew from the third VM run, so it is not a clean Mac. The trial installs nothing, so that matters less here. Claude Code 2.1.318, Opus 5.5, signed in as Ben on Ultra, with stock settings (`theme` and `tui` only). Claude Code ran in a new, empty folder `~/trial-cc`, inside tmux started in the GUI session (`sudo launchctl asuser 501 sudo -u admin tmux new-session …`), so the browser could open. I typed the stand-in user's words; Ben clicked the page through VNC.

**What happened, in order:**

1. The URL alone, with no other words. The agent fetched the repo, read `SETUP.md`, checked `uname -m` and `sw_vers`, then listed what would change in plain words and asked for an OK. This is what `SETUP.md` asks for.
2. "Yes, go ahead". The agent downloaded the binary. The checksum matched, and the signature verified with TeamIdentifier `QJ3Q58XVEE`. `spctl -a -vvv -t open --context context:primary-signature` printed `rejected`, with `source=Notarized Developer ID` and exit 3. The agent went on, as the playbook allows. The same result in the VM by hand. This VM's `spctl` rejected a good `.pkg` the same way before ([VM runs](2026-11-05-vm-run-findings.md)), so it's a VM fault, not a fault in the file.
3. **Auto mode blocked the first run of the binary** (`Blocked by auto mode … [Code from External]`). Auto mode was on by default for this account. The agent stopped, said it wouldn't try to get around the block, and offered two ways forward: a Bash permission rule, or a `!` command.
4. "I approve running the bootstrap program. Please continue." **That was enough.** The agent started the page with `--agent claude-code`, showed `welcome`, and waited.
5. Ben answered `see-plan` and then the list (`lets-go` with all nine rows ticked). The agent ran `stop`, and summarized the trial correctly: what was chosen, what the program had found already present, and that `~/.bootstrap` is all that's left.

**The page felt slow after each click.** From `~/.bootstrap/session.jsonl` in the VM:

| Event | Time (UTC) | Gap |
|---|---|---|
| answer `see-plan` | 21:49:03.05 | |
| `show list` | 21:49:11.55 | 8.5 s |
| answer `lets-go` | 21:49:20.30 | |
| `stop` | 21:49:29.81 | 9.5 s |

Each gap is one agent round trip: `wait` returns, the model reads the answer, then it runs the next command. Meanwhile the page only disables the buttons and speeds up the blob (`internal/page/files/page.js`, `answer`). Ben: "Another delay on Let's go."

## OpenCode, on the Mini

**Setup.** The Mac Mini itself (macOS 26.6.2), as `ben`, in a new folder `~/trial-oc`. OpenCode 2.2.5 from Homebrew, run as `opencode --standalone` inside tmux started over ssh. The model was OpenCode's default, **Crouton Alpha Free**, which is what a new user with no keys gets. Homebrew on the Mini first refused to run until Ben accepted the Xcode license there.

**What happened, in order:**

1. The URL alone. The agent fetched the repo, read `SETUP.md`, and asked a yes/no question through OpenCode's question tool. The question listed the checks, the download, the local page and the undo.
2. "Yes, proceed". The checksum, signature and TeamIdentifier passed, and `spctl` accepted the file (`source=Notarized Developer ID`; this Mac reached Apple's service). **Nothing blocked the run.** OpenCode asked no permission of its own.
3. `serve --agent opencode` exited 5 with "the browser didn't open". That was my setup's fault: tmux started over ssh is outside the GUI session. The agent did what the `next:` line said and gave the person the URL.
4. **The agent never ran `wait`.** After `show welcome`, it told the person to "tell me here when you've answered". Ben answered at 16:04:18. The last heartbeat was at 15:59:37, so the page rightly showed "Whoops! We lost your OpenCode session." The trial `SETUP.md` invites this reading: "show the `welcome` screen, wait for the answer".
5. "continue", as the lost screen says. The agent ran `wait answer`, found `see-plan`, showed the list, and again asked to be told. A second "continue" after the list ran `stop`, and the agent gave a correct summary.

So a weak model gets through, by way of the lost screen and two "continue"s. The recovery path works, but the experience is poor.

## What this means

- **`SETUP.md` should ask for approval to run the program in its first message,** next to what will change, so the person's yes is already in the chat when auto mode judges the first run. Untested: whether that wording passes auto mode on its own, and what a Pro account's default mode does (likely a permission prompt instead of a block).
- **The page responds immediately to every click** (Ben, 2026-11-05: "the front-end should respond immediately, even if we have a 'wait' state"). The answered screen leaves at once, and a waiting state holds until the next `show`.
- **Maybe, too: queue the next screen** when it doesn't depend on the answer (Ben: "Maybe 3?"), so the page moves on before the agent's turn. That's a new verb and needs a design pass first.

- **The deeper problem is the agent driving every step.** It only acts during its own turn, so the page works only while the agent sits in `wait`. A strong model does that; a weak one doesn't. Ben, 2026-11-05: "Could we just have the agent kick off the install process, and then have our Go server drive it until it exits?" Auto mode is no reason against that ("We already got dinged by auto mode anyway"). Recovery goes through output and errors that tell the agent what to do (`project/agents/cli-design.md`, "Errors that teach"). A design proposal follows as its own dated doc.

## Still open

- A Mac with auto mode off, where the person sees a permission prompt instead.
- OpenCode with a stronger model, and with the browser opening from a GUI session.
