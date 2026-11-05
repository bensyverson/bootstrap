# Setting up a Mac with bootstrap

You are a coding agent, and the person you're working with pasted a link to [bensyverson/bootstrap](https://github.com/bensyverson/bootstrap). This file is your playbook. Read it raw, since a summary can drop a step: `https://raw.githubusercontent.com/bensyverson/bootstrap/main/SETUP.md`. If your fetch tool summarizes pages, fetch it with `curl -fsSL` instead.

The person may never have used a terminal. Speak plainly, keep each message short, and never ask them to type a command.

## What this is

`bootstrap` is a small program by Ben Syverson, signed with his Apple Developer ID and notarized by Apple. You download it, check it, and start it once. From then on **the program runs the setup itself**: it opens a page in the person's browser, asks them each question there, and installs what they pick. Your job is to start it, tell the person what it's doing, and step in when it hands something to you.

## 1. Ask first

Before you download anything, send the person one message that says what will happen, and wait for a yes. Something like:

> I can set up this Mac the way Ben Syverson works. To do that, I'll download his small setup program, check that it's really his and approved by Apple, and run it. It opens a page in your browser where you pick what gets installed: Apple's developer tools, Homebrew, and a few of Ben's tools. Then it makes a project folder for you. It installs software, and nothing is installed that you don't pick on the page. It never asks for your password in the chat; if your Mac asks, that's macOS itself. OK to go ahead?

Say it in your own words if you like, but keep every fact: it downloads a program, checks it, runs it, and installs software the person picks. Don't go on without a clear yes in the chat. Some apps (Claude Code's auto mode) only let a downloaded program run once the person has said yes in the chat.

To undo the program later: `~/.bootstrap/bin/bootstrap stop`, then `rm -rf ~/.bootstrap`. What it installed stays.

## 2. Check this Mac

```sh
uname -m
sw_vers -productVersion
```

`bootstrap` needs Apple silicon (`arm64`) and macOS 15 or later. If either doesn't hold, tell the person in one sentence that this setup is for newer Macs, and stop.

## 3. Fetch the pinned file

This version and its checksum are pinned here on purpose. Don't swap in a newer release.

```sh
mkdir -p ~/.bootstrap/bin
curl -fsSL -o ~/.bootstrap/bin/bootstrap.download https://github.com/bensyverson/bootstrap/releases/download/v0.1.0/bootstrap
```

## 4. Check it before running it

You can't read a binary, so these checks are how you know it's the file Ben released.

```sh
echo "d6593d3de0c497ecf157fb4b0585109926f8e9da30a06093e048fc3ca2f2a60c  $HOME/.bootstrap/bin/bootstrap.download" | shasum -a 256 -c
codesign --verify --strict ~/.bootstrap/bin/bootstrap.download
codesign -dv ~/.bootstrap/bin/bootstrap.download 2>&1 | grep TeamIdentifier
spctl -a -vvv -t open --context context:primary-signature ~/.bootstrap/bin/bootstrap.download
```

- **The checksum must say `OK`.** If it doesn't, delete the file, tell the person, and stop.
- **The signature must verify, and `TeamIdentifier` must be `QJ3Q58XVEE`** (Syverson Industries LLC). If not, delete the file, tell the person, and stop.
- **Gatekeeper (`spctl`) should say `accepted` and `source=Notarized Developer ID`.** Use the command as written: the more common `spctl -a -t exec` rejects every command-line program, because it only accepts apps. Gatekeeper checks with Apple over the network, and it has rejected a good file before. So if the checksum and signature passed, a `spctl` failure alone isn't a reason to stop: tell the person what it said, and go on.

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

(or `--agent opencode`). It returns at once with `running url=… pid=…` and `next: bootstrap wait`. Tell the person a page just opened in their browser and that they'll make their choices there.

- **`the browser didn't open, but the setup is running`** (exit 5): give the person the `url` from the output and ask them to open it. If they can't, run `~/.bootstrap/bin/bootstrap stop` and use [the chat-only setup](#the-chat-only-setup).
- **Any other failure to start**: tell the person in one sentence, then use [the chat-only setup](#the-chat-only-setup).

## 6. Wait, retell, repeat

```sh
~/.bootstrap/bin/bootstrap wait
```

`wait` blocks until the setup needs you, or for 90 seconds, then prints what happened since your last wait. **Run it again every time, until it says the setup is over.** Every output ends with `next:` lines; follow them. Don't pass a longer `--timeout`: your shell tool may cut a command off at 2 minutes.

- **Exit 3, `still working, run wait again`**, is normal: an install is running, or the person is reading. Run `wait` again straight away.
- **`note …` rows** are things to retell, such as `note installed sleepy 0.2.0` or `note person chose website on making`. Pass on what matters in a short sentence, not every row. The person sees each step on the page too, so you needn't narrate everything.
- **`step …` rows** say where the setup is: `step tools started 4 of 13`.
- **Never ask the person to tell you when they've clicked.** The program hears every click itself.

## 7. When the setup hands something to you

A `handoff` row means the setup needs you. It says what happened (`what:`), shows the evidence (`command:`, `limit:`, `log:`, and the output's last lines), and ends with one `next:` line per way forward. While it waits, the page tells the person "Let's talk in" your app's name.

- **`handoff trouble`**: something went wrong. Read the log it names, and fix what you safely can. Then `~/.bootstrap/bin/bootstrap resume --retry` runs the step again from its check. If the person would rather go on without that tool or step, and the handoff offers it, `resume --skip`. If you need to talk it over, ask the person in the chat first, then resume. After a resume, go back to `wait`.
- **`handoff phone`**: the person wants to use you from their phone. Explain how your app does that (in Claude Code, `/remote-control`), then `wait` again.
- **`handoff done`**: the setup is over. Sum up in the chat what was set up, and tell the person to open a new Terminal window, go to their project folder (the handoff names it), and start you there. Nothing more to run; closing the page ends the program.

Other ways `wait` can end, each exit 5:

- **`session ended`**, **`the page is closed, so the setup isn't running`** or **`the setup stopped: …`**: run `~/.bootstrap/bin/bootstrap run --agent <your app>` again. It picks up where it left off and skips what's done. If it stops again the same way, tell the person and use [the chat-only setup](#the-chat-only-setup) for the steps that are left.

## Notes for each app

**Claude Code.** In auto mode, the first run of a downloaded program is blocked until the person has said yes in the chat, which is why step 1 asks. If a later command is blocked anyway, tell the person what it is and ask them to approve it. Run each `wait` as its own command, in the foreground; its 90-second budget is under the Bash tool's 2-minute default.

**OpenCode.** Its shell tool also stops a command at 2 minutes by default, so the same `wait` loop fits. Some models stop after one `wait` and hand the turn back: don't. Keep looping until a `handoff` or the end, even when nothing seems to change. OpenCode has no auto mode, so the person may be asked to allow each command; say why before the first one.

## The chat-only setup

Use this only when the program can't run on this Mac, or the page won't open. It is the same setup the program runs, step by step, with you asking the questions in the chat instead of the page. The tools, what each one is for, and which may be skipped are in `https://raw.githubusercontent.com/bensyverson/bootstrap/main/catalog/tools.json` (read it raw).

Rules for every step:

- **Check before you act**, so running a step twice changes nothing.
- **Call Homebrew by its full path,** `/opt/homebrew/bin/brew`, and the tools it installs the same way (`/opt/homebrew/bin/job`). A shell that started before Homebrew was installed can't find them by name. Never add `brew shellenv` to the person's shell files.
- **Before each install, say what the tool is and why**, in one sentence, from its `say` in the catalog.
- **Never ask for the person's password in the chat.** If macOS asks, it asks in its own window.
- **Never move what's already in the starting folder.**

### 1. The plan (`plan`)

Tell the person what's ahead in a few sentences: Apple's tools, Homebrew, Ben's tools, a project folder with Git, GitHub if they want it, their name on their work, and the rules that teach you how they work. List the tools from the catalog; those with `"required": false` may be left out. Ask which of those they want.

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

Run `/opt/homebrew/bin/brew trust bensyverson/tap` first, or the installs are refused. Then, for each tool the person kept that has a `formula`, and whose `detect` path doesn't exist yet: `/opt/homebrew/bin/brew install bensyverson/tap/<formula>`. If one fails, show the person the last lines and ask whether to try again or go on without it; required tools can't be left out.

### 5. The project folder (`folder`)

If the starting folder is already a Git project, and isn't the home folder, Desktop, Documents, Downloads or a folder with a generic name, set up right there. Otherwise ask for a project name and use `~/Projects/<name>`, making it if needed (`~/git` is the other common choice). From here on, work in that folder.

### 6. Git (`git`)

Done if there's a `.git` already. Otherwise ask whether Git should keep track of the project. If yes: set `git config --global init.defaultBranch main` when it's unset, then `git init`. Don't make a commit for them.

### 7. GitHub (`github`)

Done if `/opt/homebrew/bin/gh auth status` succeeds. Otherwise ask whether they want GitHub, which keeps a copy of their work online. If not, skip it. If they have no account, send them to `https://github.com/signup` first. Install `gh` if needed (`/opt/homebrew/bin/brew install gh`), then run `/opt/homebrew/bin/gh auth login --hostname github.com --web --git-protocol https --skip-ssh-key`. It prints a one-time code: give the person the code and `https://github.com/login/device`, and ask them to enter it there. Afterwards, `gh auth status` should succeed; then run `/opt/homebrew/bin/gh auth setup-git`.

### 8. Their name on their work (`identity`)

Only with a Git repository, and only if `git config --global user.name` or `user.email` is empty. After a GitHub sign-in, offer their GitHub name (or login, if the name is blank) and private email: `gh api user` gives `id` and `login`, and the email is `<id>+<login>@users.noreply.github.com`. Without GitHub, ask for a name and an email, and say the email ends up in the project's history. They can skip this. Set only the half that's empty, with `git config --global`.

### 9. What they're making (`stage`)

Ask what they're making (a website, an app, or something else) and whether it's live yet. For an app, ask whether they have Xcode; if not, offer to open it in the App Store.

### 10. The rules (`rules`)

In the project folder:

- `agents init` (`/opt/homebrew/bin/agents init`), unless there's an `AGENTS.md`. If it refuses because of the project's own `CLAUDE.md` and names `agents migrate`, run `agents migrate`, then `agents init` again.
- Write a `.gitignore` with what this kind of project shouldn't keep, adding only lines that aren't there: `.DS_Store` always; `node_modules/`, `.env` and `dist/` for a website; `.build/`, `.swiftpm/`, `DerivedData/` and `xcuserdata/` for an app.
- Make a `local/` folder, and add `local/` to `.gitignore`. Tell the person `local/` is theirs: private notes and files that are never committed.
- `job init --as claude` (or `--as opencode`), then `job gitignore`, unless there's a `.jobs.db` already. Never run `job init` over one: it would lose its tasks.

### 11. Machine settings (`settings`)

Ask before changing anything machine-wide. Then run each tool's own `hook install`, if the installed version has one.

### 12. Your phone (`phone`)

Ask whether they'd like to use you from their phone. If so, explain how your app does it (in Claude Code, `/remote-control`).

### 13. Done (`done`)

Sum up what was set up, and tell the person to open a new Terminal window, go to the project folder, and start you there, so the new tools are found by name.
