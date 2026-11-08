# Setting up a Mac with bootstrap

You are a coding agent, and the person you're working with pasted a link to [bensyverson/bootstrap](https://github.com/bensyverson/bootstrap). This file is your playbook. Read it raw, since a summary can drop a step: `https://raw.githubusercontent.com/bensyverson/bootstrap/main/SETUP.md`. Fetch it with `curl -fsSL`: a fetch tool may summarize it or refuse to give it whole. Don't use `gh` or `git` for this: on a new Mac `gh` isn't installed, and `/usr/bin/git` is a stand-in that pops up Apple's installer.

The person may never have used a terminal. Speak plainly, keep each message short, and never ask them to type a command during the setup. You run every command yourself. The only exceptions, each given as text to paste:

- starting you again in the project at the end ([below](#starting-again-in-the-project));
- `/remote-control` (Claude Code only), typed into this chat once the setup is over, if they want to use you from their phone;
- `bootstrap new`, for their later projects;
- a command they'd rather run themselves than approve for you ([Notes for each app](#notes-for-each-app)). They paste it into a new Terminal window (Command-N in Terminal), since the window you're in is busy running you.

Never run `sudo`, and never ask for their password in the chat. Never script other apps (`osascript`, System Events) to bring a window forward: macOS stops to ask the person for permission. If a window is hidden, tell them where to find it (Command-Tab, or the Dock). Anything that needs an administrator's password goes through macOS's own window.

## What this is

`bootstrap` is a small program by Ben Syverson, signed with his Apple Developer ID and notarized by Apple. You download it, check it, and start it once. From then on **the program runs the setup itself**: it opens a page in the person's browser, asks them each question there, and installs what they pick. Your job is to start it, tell the person what it's doing, and step in when it hands something to you.

## 1. Ask first

Before you download anything, send the person one short message that says what will happen, and wait for a yes. Something like:

> I can set up this Mac the way Ben Syverson works. I'll download his small setup program, check that it's really his and that Apple has scanned it, and run it. It installs software: a page in your browser lets you pick what gets installed.
>
> OK to go ahead, or want the details first?

Say it in your own words if you like, but keep every fact: it downloads a program, checks it, runs it, and installs software the person picks. If they want the details, tell them:

- The page offers Apple's developer tools, Homebrew (a tool that installs other tools), and a few of Ben's tools, each with a line on what it's for. Nothing is installed that they don't pick.
- Then it makes a project folder for them, or sets up the one you started in.
- With their OK on the page, it also signs them in to GitHub, sets the name and email Git puts on their work, and turns on a few machine-wide helpers for you. The helpers include a short note in your instructions for every session (`~/.claude/CLAUDE.md`, or OpenCode's `~/.config/opencode/AGENTS.md`) saying how to start a new project; everything else in that file stays.
- It never asks for their password in the chat. If their Mac asks, that's macOS itself.

Then ask again. Don't go on without a clear yes in the chat. Some apps (Claude Code's auto mode) only let a downloaded program run once the person has said yes in the chat.

To undo the program later: `~/.bootstrap/bin/bootstrap stop`, then `rm -rf ~/.bootstrap`. Once the setup has moved it into Homebrew, it's `/opt/homebrew/bin/bootstrap stop`, then `brew uninstall bootstrap` and `rm -rf ~/.bootstrap`. What it installed stays. The note in your instructions file sits between `<!-- bootstrap:begin -->` and `<!-- bootstrap:end -->`; delete those lines and what's between them to take it out.

## 2. Check this Mac

```sh
sysctl -n hw.optional.arm64
sw_vers -productVersion
```

`bootstrap` needs Apple silicon (the first line prints `1`; `uname -m` can say `x86_64` under Rosetta, so don't use it) and macOS 15 or later. If either doesn't hold, tell the person in one sentence that this setup is for newer Macs, and stop.

```sh
id -Gn | grep -qw admin && echo admin
```

Apple's tools and Homebrew's installer need an administrator's password. If that prints nothing, this account isn't an administrator: tell the person the setup needs one (System Settings ▸ Users & Groups shows which accounts are), and stop.

## 3. Fetch the pinned file

This version and its checksum are pinned here on purpose. Don't swap in a newer release.

```sh
mkdir -p ~/.bootstrap/bin
curl -fsSL -o ~/.bootstrap/bin/bootstrap.download https://github.com/bensyverson/bootstrap/releases/download/v0.3.2/bootstrap
```

If the download fails, try once more. If it fails again, see [When the program can't go on](#when-the-program-cant-go-on). In Claude Code, `Operation not permitted` here means its sandbox is on; see [Notes for each app](#notes-for-each-app).

## 4. Check it before running it

You can't read a binary, so these checks are how you know it's the file Ben released.

```sh
echo "e8ab1259891dbec85253950daf7ec304d9e0b6584f6d71c0bc005798a019dd7c  $HOME/.bootstrap/bin/bootstrap.download" | shasum -a 256 -c
codesign --verify --strict ~/.bootstrap/bin/bootstrap.download
codesign -dv ~/.bootstrap/bin/bootstrap.download 2>&1 | grep TeamIdentifier
spctl -a -vvv -t open --context context:primary-signature ~/.bootstrap/bin/bootstrap.download
```

- **The checksum must say `OK`.** If it doesn't, delete the file, tell the person, and stop.
- **The signature must verify, and `TeamIdentifier` must be `QJ3Q58XVEE`** (Syverson Industries LLC). If not, delete the file, tell the person, and stop.
- **Gatekeeper (`spctl`) should say `accepted` and `source=Notarized Developer ID`.** Use the command as written: the more common `spctl -a -t exec` rejects every command-line program, because it only accepts apps. Gatekeeper checks with Apple over the network, and it has rejected a good file before. So if the checksum and signature passed, a `spctl` failure alone isn't a reason to stop. Tell the person plainly: the file matches the one Ben released and carries his signature, but Apple's online check didn't answer as expected. Then ask whether to go on, since your first message promised Apple's check.

Then make it runnable:

```sh
mv ~/.bootstrap/bin/bootstrap.download ~/.bootstrap/bin/bootstrap
chmod +x ~/.bootstrap/bin/bootstrap
```

## 5. Start the setup

Stay in the folder the person started you in: the program looks at it to decide where the project goes. Call the program by its full path, since it isn't on `PATH`, and say which app you are:

```sh
~/.bootstrap/bin/bootstrap run --agent claude-code
```

(or `--agent opencode`; [the table below](#notes-for-each-app) has each app's names). It returns at once with `running url=… pid=…` and `next: bootstrap wait`. Tell the person a page just opened in their browser and that they'll make their choices there.

- **`the browser didn't open, but the setup is running`** (exit 5): give the person the `url` from the output and ask them to open it. If they can't, run `~/.bootstrap/bin/bootstrap stop` and see [When the program can't go on](#when-the-program-cant-go-on).
- **Any other failure to start**: see [When the program can't go on](#when-the-program-cant-go-on).

## 6. Wait, retell, repeat

```sh
~/.bootstrap/bin/bootstrap wait
```

`wait` blocks until the setup needs you, or for 90 seconds, then prints what happened since your last wait. **Run it again every time, until it says the setup is over.** Every output ends with `next:` lines; follow them. They name the program by its full path, so run them as written.

**The program moves into Homebrew partway through.** In the tools step, once Homebrew is in, the setup installs bootstrap from Homebrew too, so `brew upgrade` keeps it current, and deletes `~/.bootstrap/bin/bootstrap`. From then on the `next:` lines name `/opt/homebrew/bin/bootstrap`: use that path wherever this page says `~/.bootstrap/bin/bootstrap`. If Homebrew's copy can't be installed, a `note kept ~/.bootstrap/bin/bootstrap` row says so, and the download keeps running the setup. Nothing for you to fix. Don't pass a longer `--timeout`: your shell tool may cut a command off at 2 minutes.

- **Exit 0** means it has something for you: rows to retell, a `handoff` (below), or the end of the setup.
- **Exit 3, `still working, run wait again`**, is normal: an install is running, or the person is reading. Run `wait` again straight away.
- **Exit 5** is covered [below](#7-when-the-setup-hands-something-to-you). **Any other exit**, or no output at all: run `wait` once more. If it happens again, see [When the program can't go on](#when-the-program-cant-go-on).
- **`note …` rows** are things to retell, such as `note installed sleepy 0.2.0` or `note person chose website on making`. Pass on what matters in a short sentence, not every row. The person sees each step on the page too, so you needn't narrate everything.
- **`step …` rows** say where the setup is: `step tools started 4 of 13`.
- **Never ask the person to tell you when they've clicked.** The program hears every click itself.

If the person says in the chat at any point that they want to stop, run `~/.bootstrap/bin/bootstrap stop` and end. What's installed stays.

## 7. When the setup hands something to you

A `handoff` row means the setup needs you. It says what happened (`what:`), shows the evidence (`command:`, `limit:`, `log:`, and the output's last lines), and ends with one `next:` line per way forward. While it waits, the page tells the person "Let's talk in" your app's name.

- **`handoff trouble`**: something went wrong. Read the log it names, and fix what you safely can, without `sudo` or the person's password. Then `~/.bootstrap/bin/bootstrap resume --retry` runs the step again from its check. If the person would rather go on without that tool or step, and the handoff offers it, `resume --skip`. If you need to talk it over, ask the person in the chat first, then resume. After a resume, go back to `wait`. If the cure is something the person does in another window, such as clicking Install, don't hand the turn back to ask them to tell you: check for its result in a loop that ends on its own within 90 seconds, such as `for i in $(seq 1 9); do test -e <path> && break; sleep 10; done; test -e <path>`, and run it again until the result is there (your shell tool may stop any command at 2 minutes). Then resume.
- **`handoff phone`**: the person wants to use you from their phone. Explain how your app does that (in Claude Code, they type `/remote-control` into this chat once the setup is over), then `wait` again. OpenCode never gets this handoff: it has no way of its own to reach a phone, so the setup skips the question there.
- **`handoff done`**: the setup is over. Sum up in the chat what was set up, then tell the person how to start you again in their project folder (the handoff names it), so the new tools are found by name. See [Starting again in the project](#starting-again-in-the-project). Nothing more to run, not even `stop`: closing the page ends the program. If they'll make more projects and the handoff says your instructions now say how, tell them they can just ask you next time. Either way, they can skip the page: in a new Terminal window, `bootstrap new <name>` (or `~/.bootstrap/bin/bootstrap new <name>` if a `note kept` row said it stayed there) sets up a new project and asks only what it can't tell on its own.

Other ways `wait` can end, each exit 5:

- **`the page is closed, so the setup isn't running`**: the person may have closed it to stop. Ask whether they want to carry on. If not, run `~/.bootstrap/bin/bootstrap stop`, tell them what's already installed stays, and end. If so, carry on as below.
- **`session ended`**, **`the setup stopped: …`**, or a closed page they want to carry on from: run `~/.bootstrap/bin/bootstrap run --agent claude-code` (or `opencode`) again. It picks up where it left off and skips what's done. If it stops again the same way, see [When the program can't go on](#when-the-program-cant-go-on).

## Starting again in the project

The setup ends with the person starting you afresh in their project folder, in a new Terminal window. Homebrew's installer puts `/opt/homebrew/bin` on the `PATH` of every Terminal window opened after it (through `/etc/paths.d/homebrew`), so a new window finds the tools by name; restarting you in the old window doesn't. Never add Homebrew to `.zprofile` or `.zshrc`: it isn't needed. Tell them how, for the app they're using, and give each line as text to paste:

- **Claude Code in Terminal:** type `/exit` to end this session (or press Control-C twice). Press Command-N in Terminal for a new window, and paste these two lines (with their folder):

  ```sh
  cd ~/Projects/pine-bakery
  claude
  ```

- **The Claude desktop app, or an editor:** quit the app fully (Command-Q) and open it again, so it picks up the new `PATH`, then start a new session and pick the project folder when it asks for one.
- **OpenCode:** type `/exit`, press Command-N, then `cd` to the folder and run `opencode`, as above.

## Notes for each app

Each app goes by a few names in these commands:

| | Claude Code (in Terminal, the Claude desktop app, or an editor) | OpenCode |
|---|---|---|
| `bootstrap run --agent` | `claude-code` | `opencode` |
| `job init --as` | `claude` | `opencode` |
| `hook install --harness` | `claude-code` | `opencode` |
| `{ai}` in the catalog's `say` lines | Claude | OpenCode |

**Claude Code.** In auto mode, the first run of a downloaded program is blocked until the person has said yes in the chat, which is why step 1 asks. If a later command is blocked anyway, tell the person what it is and ask them to approve it. Run each `wait` as its own command, in the foreground; its 90-second budget is under the Bash tool's 2-minute default.

Claude Code may also run your commands in a sandbox (the person turns it on with `/sandbox`). Inside it, commands can't write to `~/.bootstrap`, open apps or files with `open`, or reach Apple's signature service, so `mkdir` fails with `Operation not permitted`, `open` fails, and Apple's checks can call a good file invalid. Run the downloads, the checks in step 4 and every `bootstrap` command outside the sandbox. Claude Code asks the person to approve each one; say what it is first. `wait` runs every 90 seconds while they're on the page, so at the first prompt for `~/.bootstrap/bin/bootstrap`, and again when the program moves to `/opt/homebrew/bin/bootstrap`, ask them to pick the option that stops asking for that command; otherwise the loop stalls on a prompt they can't see. If they'd rather not approve something, give them the exact command as text to paste into a new Terminal window.

**OpenCode.** It has no `/remote-control`, so the setup never asks about the phone; if the person asks, say so plainly rather than inventing a way. Its shell tool also stops a command at 2 minutes by default, so the same `wait` loop fits. Some models stop after one `wait` and hand the turn back: don't. Keep looping until a `handoff` or the end, even when nothing seems to change. OpenCode has no auto mode, so the person may be asked to allow each command; say why before the first one.

## When the program can't go on

If the program can't be fetched or started, or it stops at the same place twice after `run` or `resume`, don't try to do its steps yourself. Run `~/.bootstrap/bin/bootstrap stop` if it's still running (`/opt/homebrew/bin/bootstrap` once it has moved there), then tell the person, in a few plain sentences:

- the setup couldn't finish this time, and how far it got (`bootstrap status` names the screen it was on, if it still answers);
- everything already installed stays, and works;
- pasting the setup link into you again later picks up where it left off;
- if they'd like to tell Ben, https://github.com/bensyverson/bootstrap/issues takes reports (it needs a GitHub account). Offer to write one for them: what happened, the last lines of the log, their macOS version and your app.

Then stop.
