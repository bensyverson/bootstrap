# Setting up a Mac with bootstrap

You are a coding agent, and the person you're working with pasted a link to [bensyverson/bootstrap](https://github.com/bensyverson/bootstrap). This file is your playbook. Read it raw, since a summary can drop a step: `https://raw.githubusercontent.com/bensyverson/bootstrap/main/SETUP.md`. Fetch it with `curl -fsSL`: a fetch tool may summarize it or refuse to give it whole. Don't use `gh` or `git` for this: on a new Mac `gh` isn't installed, and `/usr/bin/git` is a stand-in that pops up Apple's installer.

The person may never have used a terminal. Speak plainly, keep each message short, and never ask them to type a command during the setup. You run every command yourself. The only exceptions, each given as text to paste:

- starting you again in the project at the end ([below](#starting-again-in-the-project));
- `/remote-control`, typed into this chat once the setup is over, if they want to use you from their phone;
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
- With their OK on the page, it also signs them in to GitHub, sets the name and email Git puts on their work, and turns on a few machine-wide helpers for you.
- It never asks for their password in the chat. If their Mac asks, that's macOS itself.

Then ask again. Don't go on without a clear yes in the chat. Some apps (Claude Code's auto mode) only let a downloaded program run once the person has said yes in the chat.

To undo the program later: `~/.bootstrap/bin/bootstrap stop`, then `rm -rf ~/.bootstrap`. What it installed stays.

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
curl -fsSL -o ~/.bootstrap/bin/bootstrap.download https://github.com/bensyverson/bootstrap/releases/download/v0.2.0/bootstrap
```

If the download fails, try once more. If it fails again, tell the person in one sentence and use [the chat-only setup](#the-chat-only-setup). In Claude Code, `Operation not permitted` here means its sandbox is on; see [Notes for each app](#notes-for-each-app).

## 4. Check it before running it

You can't read a binary, so these checks are how you know it's the file Ben released.

```sh
echo "c85f9dbc22b2f8fe65516d503ed7155c6865bacfef83100e7c90536e5ea50ae8  $HOME/.bootstrap/bin/bootstrap.download" | shasum -a 256 -c
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

- **`the browser didn't open, but the setup is running`** (exit 5): give the person the `url` from the output and ask them to open it. If they can't, run `~/.bootstrap/bin/bootstrap stop` and use [the chat-only setup](#the-chat-only-setup).
- **Any other failure to start**: tell the person in one sentence, then use [the chat-only setup](#the-chat-only-setup).

## 6. Wait, retell, repeat

```sh
~/.bootstrap/bin/bootstrap wait
```

`wait` blocks until the setup needs you, or for 90 seconds, then prints what happened since your last wait. **Run it again every time, until it says the setup is over.** Every output ends with `next:` lines; follow them. They name the program as plain `bootstrap`, but it isn't on `PATH`, so always call it as `~/.bootstrap/bin/bootstrap`. Don't pass a longer `--timeout`: your shell tool may cut a command off at 2 minutes.

- **Exit 0** means it has something for you: rows to retell, a `handoff` (below), or the end of the setup.
- **Exit 3, `still working, run wait again`**, is normal: an install is running, or the person is reading. Run `wait` again straight away.
- **Exit 5** is covered [below](#7-when-the-setup-hands-something-to-you). **Any other exit**, or no output at all: run `wait` once more. If it happens again, tell the person in one sentence and use [the chat-only setup](#the-chat-only-setup).
- **`note …` rows** are things to retell, such as `note installed sleepy 0.2.0` or `note person chose website on making`. Pass on what matters in a short sentence, not every row. The person sees each step on the page too, so you needn't narrate everything.
- **`step …` rows** say where the setup is: `step tools started 4 of 13`.
- **Never ask the person to tell you when they've clicked.** The program hears every click itself.

If the person says in the chat at any point that they want to stop, run `~/.bootstrap/bin/bootstrap stop` and end. What's installed stays.

## 7. When the setup hands something to you

A `handoff` row means the setup needs you. It says what happened (`what:`), shows the evidence (`command:`, `limit:`, `log:`, and the output's last lines), and ends with one `next:` line per way forward. While it waits, the page tells the person "Let's talk in" your app's name.

- **`handoff trouble`**: something went wrong. Read the log it names, and fix what you safely can, without `sudo` or the person's password. Then `~/.bootstrap/bin/bootstrap resume --retry` runs the step again from its check. If the person would rather go on without that tool or step, and the handoff offers it, `resume --skip`. If you need to talk it over, ask the person in the chat first, then resume. After a resume, go back to `wait`. If the cure is something the person does in another window, such as clicking Install, don't hand the turn back to ask them to tell you: check for its result in a loop, as [the chat-only setup's waiting rule](#the-chat-only-setup) describes, then resume.
- **`handoff phone`**: the person wants to use you from their phone. Explain how your app does that (in Claude Code, they type `/remote-control` into this chat once the setup is over), then `wait` again.
- **`handoff done`**: the setup is over. Sum up in the chat what was set up, then tell the person how to start you again in their project folder (the handoff names it), so the new tools are found by name. See [Starting again in the project](#starting-again-in-the-project). Nothing more to run, not even `stop`: closing the page ends the program. If they'll make more projects, tell them that next time they can skip the page: in Terminal, `~/.bootstrap/bin/bootstrap new <name>` sets up a new project and asks only what it can't tell on its own.

Other ways `wait` can end, each exit 5:

- **`the page is closed, so the setup isn't running`**: the person may have closed it to stop. Ask whether they want to carry on. If not, run `~/.bootstrap/bin/bootstrap stop`, tell them what's already installed stays, and end. If so, carry on as below.
- **`session ended`**, **`the setup stopped: …`**, or a closed page they want to carry on from: run `~/.bootstrap/bin/bootstrap run --agent claude-code` (or `opencode`) again. It picks up where it left off and skips what's done. If it stops again the same way, tell the person and use [the chat-only setup](#the-chat-only-setup) for the steps that are left.

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

Claude Code may also run your commands in a sandbox (the person turns it on with `/sandbox`). Inside it, commands can't write to `~/.bootstrap`, open apps or files with `open`, or reach Apple's signature service, so `mkdir` fails with `Operation not permitted`, `open` fails, and `pkgutil --check-signature` calls a good package invalid. Run the downloads, the checks in step 4 and every `bootstrap` command outside the sandbox, and in the chat-only setup, every command (it writes outside the starting folder, to `~/.gitconfig` and to your own settings, and signs in over the network). Claude Code asks the person to approve each one; say what it is first. `wait` runs every 90 seconds while they're on the page, so at the first prompt for `~/.bootstrap/bin/bootstrap`, ask them to pick the option that stops asking for that command; otherwise the loop stalls on a prompt they can't see. If they'd rather not approve something, give them the exact command as text to paste into a new Terminal window.

**OpenCode.** It has no `/remote-control`; if the person asks about their phone, say so plainly rather than inventing a way. Its shell tool also stops a command at 2 minutes by default, so the same `wait` loop fits. Some models stop after one `wait` and hand the turn back: don't. Keep looping until a `handoff` or the end, even when nothing seems to change. OpenCode has no auto mode, so the person may be asked to allow each command; say why before the first one.

## The chat-only setup

Use this only when the download fails twice (step 3), the program won't start or keeps failing (steps 5 to 7), or the page won't open. Not when step 2 or step 4 said to stop: an older Mac, or a download that failed its checksum or signature, ends the setup. It is the same setup the program runs, step by step, with you asking the questions in the chat instead of the page. The tools, what each one is for, and which may be skipped are in `https://raw.githubusercontent.com/bensyverson/bootstrap/main/catalog/tools.json` (read it raw).

Rules for every step:

- **Check before you act**, so running a step twice changes nothing.
- **Call Homebrew by its full path,** `/opt/homebrew/bin/brew`, and the tools it installs the same way (`/opt/homebrew/bin/job`). A shell that started before Homebrew was installed can't find them by name. Never add `brew shellenv` to the person's shell files.
- **Before each install, say what the tool is and why**, in one sentence, from its `say` in the catalog. `{ai}` there stands for your name: Claude, or OpenCode.
- **Never ask for the person's password in the chat.** If macOS asks, it asks in its own window.
- **Never move what's already in the starting folder.**
- **Waiting for a click or a long install:** check in a loop that ends on its own within 90 seconds, for example `for i in $(seq 1 9); do test -e <path> && break; sleep 10; done; test -e <path>`, and run it again until the path exists. Your shell tool may stop any command at 2 minutes. Never hand the turn back to ask the person to tell you when they're done. The one exception is making a GitHub account (step 7), which leaves nothing to check for.
- **The person's choices from the page are gone** if you came here partway through the program's setup. Steps already done skip themselves; ask again only what's left. Their answers from an earlier setup are in `~/.bootstrap/preferences.json` under `answers` (and `skip_tools`, the tools they went without): offer each as the likely answer, but still ask.

### 1. The plan (`plan`)

Tell the person what's ahead in a few sentences: Apple's tools, Homebrew, Ben's tools, a project folder with Git, GitHub if they want it, their name on their work, and the rules that teach you how they work. List the tools from the catalog; those with `"required": false` may be left out. Ask which of those they want. Apple's tools and Homebrew can't be left out: everything else installs through them. If the person doesn't want either, say that the setup needs it, and stop.

### 2. Apple's tools (`apple`)

Done if `/Library/Developer/CommandLineTools/usr/bin/git` exists. (Don't test `command -v git`: `/usr/bin/git` is there before the tools are, and only asks to install them.) Otherwise check there are about 20 GB free (`df -H /`); if not, say so and stop. Run `xcode-select --install`, then `open -a "/System/Library/CoreServices/Install Command Line Developer Tools.app"` to bring its window forward. Ask the person to click **Install** and agree to the license. It takes a while; check for that `git` file every minute or so, and go on once it's there.

### 3. Homebrew (`homebrew`)

Done if `/opt/homebrew/bin/brew` exists. Don't use the command pasted from brew.sh: Apple blocks it in an agent's shell. Use Homebrew's own installer instead:

```sh
curl -fsSL -o ~/.bootstrap/Homebrew.pkg https://github.com/Homebrew/brew/releases/latest/download/Homebrew.pkg
pkgutil --check-signature ~/.bootstrap/Homebrew.pkg
```

The signature must name `927JGANW46` and say it's notarized. If it doesn't, delete the file, tell the person, and stop. Then `open ~/.bootstrap/Homebrew.pkg`, and ask the person to click through it; macOS asks for their password there, not in the chat. Go on once `/opt/homebrew/bin/brew` exists.

### 4. Ben's tools (`tools`)

Run `/opt/homebrew/bin/brew trust bensyverson/tap` first, or the installs are refused. If Homebrew answers `Unknown command`, it's an older Homebrew: run `/opt/homebrew/bin/brew update` and try `trust` again. If it's still unknown, that Homebrew doesn't ask for trust, so go on without it. Then, for each tool the person kept that has a `formula`, look at each of its `detect` paths, with `~` expanded to their home folder. If none exists: `/opt/homebrew/bin/brew install bensyverson/tap/<formula>`. A tool already there under `/opt/homebrew/` may be too old, so run `/opt/homebrew/bin/brew upgrade bensyverson/tap/<formula>` for it. A copy anywhere else (one they built themselves) is theirs, so leave it, and from then on call that tool at the path you found rather than `/opt/homebrew/bin/`. If one fails, show the person the last lines and ask whether to try again or go on without it; required tools can't be left out.

### 5. The project folder (`folder`)

If the starting folder holds a `.git` folder, and isn't the home folder, Desktop, Documents or Downloads, set up right there. Otherwise ask for a project name, or whether they'd like just the tools for now, with no project. Just the tools skips this step, Git (6), what they're making (9) and the rules (10), and keeps the steps for the whole Mac: GitHub, their name on their work, machine settings and the phone. For a project, ask where projects should go: `~/Projects` (the default) or `~/git`. If `~/.bootstrap/preferences.json` has a `projects_in`, that's where their last project went, so offer it first. Turn the name into a folder name in lowercase with dashes ("Pine Bakery" becomes `pine-bakery`: letters, digits, `.`, `_` and `-`, starting with a letter or digit), and confirm it. Use `<that folder>/<name>`, making it if needed. If it's already there and has files in it, ask whether to set up in it or pick another name.

From here on, every command works in the project folder. Your shell may go back to where it started between commands, so give the folder's full path in each one: `cd /Users/<them>/Projects/pine-bakery && …`, or `git -C <path>`.

### 6. Git (`git`)

Done if there's a `.git` already. Otherwise ask whether Git should keep track of the project. If yes: set `git config --global init.defaultBranch main` when it's unset, then `git init`. Don't make a commit for them.

### 7. GitHub (`github`)

Done if `/opt/homebrew/bin/gh auth status` succeeds. Otherwise ask whether they want GitHub, which keeps a copy of their work online. If not, skip it. If they have no account, send them to `https://github.com/signup` and wait for them to say in the chat that it's made; start the sign-in only then, since its code expires. Install `gh` if needed (`/opt/homebrew/bin/brew install gh`), then run `/opt/homebrew/bin/gh auth login --hostname github.com --web --git-protocol https --skip-ssh-key`. It prints a one-time code, and then waits until the person enters it, so it has to keep running after your command returns:

- **In Claude Code**, run it with the Bash tool's background option (`run_in_background`), adding `< /dev/null > ~/.bootstrap/gh-login.txt 2>&1`.
- **Elsewhere**, start it with `nohup`, so it outlives the command: `nohup /opt/homebrew/bin/gh auth login … < /dev/null > ~/.bootstrap/gh-login.txt 2>&1 &`.

Read the code with `grep -oE '[A-Z0-9]{4}-[A-Z0-9]{4}' ~/.bootstrap/gh-login.txt` (gh's wording around it varies; if that finds nothing, read the whole file), give the person the code and `https://github.com/login/device`, and ask them to enter it there. Then wait for the sign-in, running this again until it succeeds:

```sh
for i in $(seq 1 9); do /opt/homebrew/bin/gh auth status >/dev/null 2>&1 && break; sleep 10; done; /opt/homebrew/bin/gh auth status
```

Then run `/opt/homebrew/bin/gh auth setup-git`.

### 8. Their name on their work (`identity`)

Only with a Git repository (or with just the tools, since these settings are for the whole Mac), and only if `git config --global user.name` or `user.email` is empty. After a GitHub sign-in, offer their GitHub name (or login, if the name is blank) and private email: `/opt/homebrew/bin/gh api user` gives `id` and `login`, and the email is `<id>+<login>@users.noreply.github.com`. Without GitHub, ask for a name and an email, and say the email ends up in the project's history. They can skip this. Set only the half that's empty, with `git config --global`.

### 9. What they're making (`stage`)

Ask what they're making (a website, an app, or something else). For an app, ask whether they have Xcode; if not, offer to open it in the App Store: `open macappstore://apps.apple.com/app/xcode/id497799835` (or the web page, https://apps.apple.com/app/xcode/id497799835).

### 10. The rules (`rules`)

In the project folder:

- `/opt/homebrew/bin/agents init github.com/bensyverson/agents-md`, unless there's an `AGENTS.md`. If it refuses because of the project's own `CLAUDE.md` and names `agents migrate`, run `/opt/homebrew/bin/agents migrate`, then `agents init` again. Then add the rules for what they're making, plus `stage-build` for a new project (a project that was already there gets no stage): `/opt/homebrew/bin/agents add web stage-build` for a website, `agents add swift swiftui stage-build` for an app, `agents add stage-build` otherwise, each by its full path. Also add the rules for the languages the project's top-level files show: `go` for a `go.mod`; `python` for a `pyproject.toml`, `requirements.txt` or `setup.py`; `swift` for a `Package.swift` or an `.xcodeproj` (once, even if the app already added it). Skip `agents add` when there's nothing to add.
- Write a `.gitignore` with what this kind of project shouldn't keep, adding only lines that aren't there: `.DS_Store` always; `node_modules/`, `.env` and `dist/` for a website; `.build/`, `.swiftpm/`, `DerivedData/` and `xcuserdata/` for an app. Add the same for the languages the files show, each line once: `__pycache__/`, `.venv/`, `.pytest_cache/`, `.mypy_cache/` and `.ruff_cache/` for Python; the app's four for Swift; nothing for Go.
- Make a `local/` folder, and add `local/` to `.gitignore`. Tell the person `local/` is theirs: private notes and files that are never committed.
- `/opt/homebrew/bin/job init --as claude` in Claude Code, or `--as opencode` in OpenCode (not `claude-code`), then `/opt/homebrew/bin/job gitignore`, unless there's a `.jobs.db` already. Never run `job init` over one: it would lose its tasks.

### 11. Machine settings (`settings`)

Ask before changing anything machine-wide. Each tool edits your app's settings itself, keeps every other setting, and changes nothing on a second run. With their yes, run, using `--harness claude-code` in Claude Code or `--harness opencode` in OpenCode:

- `/opt/homebrew/bin/contextfill hook install --harness claude-code`: after each commit, it tells you how full your context is, so you know when to wrap up (33% on the main thread, 40% for a helper agent). In OpenCode it links contextfill's plugin instead.
- `/opt/homebrew/bin/job hook install --harness claude-code`: in a project that uses `job`, it turns plan mode down and points you to `job`'s task list instead. OpenCode has nothing to block, so it says so and changes nothing.

Each prints one line per change. To take one out later: the same command with `uninstall`.

### 12. Your phone (`phone`)

Ask whether they'd like to use you from their phone. If so, explain how your app does it (in Claude Code, they type `/remote-control` into the chat). OpenCode has none; say so.

### 13. Done (`done`)

Sum up what was set up, and tell the person how to start you again in the project folder, as in [Starting again in the project](#starting-again-in-the-project). With just the tools, tell them how to make a project later instead: paste the setup link into you again, or run `~/.bootstrap/bin/bootstrap new <name>` in Terminal if the program is there.
