# Setup link plan

The goal: Ben sends someone a link, they paste it into Claude Code or OpenCode, and they end up with his way of working on their machine and in their project. The agent explains as it goes, asks before it changes anything, and can be re-run safely.

**The test (Ben, 2026-11-05):** someone boots a brand-new MacBook Neo for the first time, installs Claude Code, and pastes the URL into the thread. Do they get up and running, and do they learn what is being installed and why?

## Who it's for

Mac users on Apple silicon who are comfortable pasting into an agent. They may have no Homebrew, no Git and no GitHub account, so the agent explains each step in plain words. Intel Macs and Linux are out for now, and the setup says so clearly.

## How the link works

The link is the repo URL. A test fetch of `github.com/bensyverson/agents` (WebFetch, 2026-11-05) returned about 14,000 characters: GitHub's menus, the file list, then the README. The raw README URL returned only the 2,561-character README. So the README is what an agent reads first, either way.

- **`README.md`** opens with a sentence for people, then a note for agents: someone who pasted this link probably wants setup, so read `SETUP.md`, say what will change, and wait for their OK. It lists what changes, split into this machine and this project.
- **`SETUP.md`** is the playbook. The agent handles the judgment: detecting the stack, asking questions, and filling in the `AGENTS.md` head.
- **`Brewfile`** installs the tools. Existing commands do the rest: `agents init` and `job init`.

  > **Correction, 2026-11-05:** on a new account `brew bundle` refuses the tap until `brew trust bensyverson/tap` has run. See [the VM run](2026-11-05-vm-run-findings.md).


An agent may rightly treat a page's instructions as untrusted. Stating the changes and asking first is the design, not a workaround. How agents treat a bare link is untested until the acceptance leaf.

## The interface

The agent opens a rich local page served by `bootstrap`, a small signed and notarized Go binary (Ben, 2026-11-05). We first planned a Perl script, then chose Go because the page needs live progress, embedded assets and a heartbeat ([what's stock on macOS](2026-11-05-stock-macos-runtimes.md)).

- **What it does:** it serves the page, collects choices and reports what's already on the machine. The agent still does the installing, so judgment stays with the agent. All screens and copy live in the binary.
- **How the agent talks to it:** a small command line with JSON output, along the lines of `bootstrap detect`, `serve`, `step` and `wait`.
- **How it arrives:** with `curl` from a GitHub release, since Homebrew may not exist yet. `curl` sets no quarantine flag ([research](2026-11-05-clean-mac-research.md)). The agent can't read a binary, so `SETUP.md` pins a version and checksum, and the agent verifies those and the Developer ID signature before running it. Later runs can use the tap.
- **Security:** it binds to `127.0.0.1`, requires a random token, checks `Host` and `Origin`, and exits after the agent is gone.
- **Assets:** Three.js, the card images and Grand are embedded. Grand ships only as Regular and Medium, subset to about 100 Latin characters, about 9KB each, with kerning and Ben's copyright kept.

- **Look (Ben, 2026-11-05):** magical and ethereal, in light mode, around one central Three.js object that morphs and glows. Bold titles and simple choices, with no eyebrow text, subheads or detailed breakdowns. Each screen has very few words, at a 6th-grade reading level or below. The first mock is in [2026-11-05-setup-page-mock/](2026-11-05-setup-page-mock/): an iridescent bubble that gains a light with each tool, so it also shows progress. Ben found the bubble too close to Siri, tried clear glass metaballs in a motion prototype, and chose silk threads (2026-11-05): light that adds up in linear color, moving from blue through cyan to green as work gets done, then relaxing to its starting form. Motion follows state: fast while the agent works, slow and soothing while it waits for the user. Type is Grand, Ben's own typeface: Medium for titles, tracked in slightly, and Regular for buttons. The name of the launching agent comes from the environment, so "Claude" and "Claude Code" swap to "OpenCode". The tool is called Jobs in anything a person reads, and tool names link to their repos. The first title avoids sounding like Apple. **There is never a hard cut (Ben):** every change of state animates over 1 to 2 seconds on a cubic S-curve, and titles and controls always transition in and out. Ben leans toward threads overall and loves glass's progress fill; if it's glass, the fill goes from blue to green. Apple's Command Line Tools are their own step, so nothing big is hidden inside the Homebrew step, and that step explains what it does before asking for a password. Proposed in the prototype and approved by Ben on 2026-11-05, since the prototype represents the desired design: one list screen with checkboxes instead of one approval per tool, a one-line status while working, a fill inside the object for progress, and small pictures of each tool drifting by.
- **Choice:** the page offers "Show me" or "Just give me the goods". The fast path says "Got it! Continue in Claude Code." The guided path explains what's being installed and why, offers toggles for anything skippable, and shows live progress.

  > **Correction, 2026-11-05.** The approved prototype changed these words: the first screen offers "See the plan" and "Skip the tour", and the fast path says "Got it. We'll only stop if we need you." See [the prototype](2026-11-05-setup-page-mock/prototype/).

- **Fallback:** if the script can't run, the agent says so in a sentence, then teaches and asks in the chat. There is no hosted page.

  > **Correction, 2026-11-05.** "The script" is now the signed `bootstrap` binary. The fallback itself stands.

- **One source:** what each tool is, why it's there and whether it can be skipped live in one file that the page and the agent both read.
- **`/remote-control`:** offered as the last step, because the Homebrew install needs someone at the Mac. Which accounts can use it is unchecked.
- **Pro mode:** the first run offers "skip the tour next time" and saves that in bootstrap's own preferences file. After that there's no browser and no explanations, only the questions it can't detect and a short summary. The setup can also install a `/new-project` command that fetches the latest `SETUP.md`, so `SETUP.md` has a machine part that runs once and a project part that runs every time.

  > **Correction, 2026-11-05 (Ben):** the `/new-project` command is now a user-level skill named `new-project`, so saying "start a new project" can trigger it as well as the slash command. It only fetches the latest `SETUP.md` and follows it. It's an optional row in the "Here's what you get" list rather than a later offer. The hint about using it goes on the page and in the chat.


## Required and skippable tools

- **Required:** Homebrew, Git, `agents`, `job`, `contextfill`.
- **Skippable:** `sleepy`, `peep`, `woodcase`, `gh`, the `contextfill` hook, the plan-mode block and the `PATH` change.

Some modules name skippable tools: `web.md` relies on `sleepy`, and `images.md` lists `peep`, `sleepy` and `woodcase`. So modules decide what's recommended: enabling `web` recommends `sleepy`, and enabling `images` recommends all three. Skipping one says what the agent loses, and a later project that enables the module offers the tool again. The rules also need to say what to do when a tool isn't installed.

## Setup order

Each step skips what's already done, so running it again is safe. Every install says what the tool is and why it's being installed before it happens.

> **Correction, Ben, 2026-11-05.** The `bootstrap` server now runs these steps, not the agent, and step 0 moves to the start of the project part (step 5), since only the project steps care where they run. GitHub (step 4) moves into the software too. See [the server-driven flow](2026-11-05-server-driven-flow.md).

0. **Where are we?** On a new Mac, Claude Code may well be running in the home folder. Never run `git init` or `agents init` there: offer to create a project folder and have the user restart the session in it.

   > **Addition, Ben, 2026-11-05.** The same goes for `~/Desktop`, `~/Documents`, `~/Downloads` and generically named folders like "Claude" or "Claude Code". `bootstrap detect` reports the folder's kind. See [the binary plan](2026-11-05-bootstrap-binary-plan.md).

1. **Homebrew.** If it's missing, explain it and offer a walkthrough. The user runs the official brew.sh command in a real terminal, because it can ask for their Mac password and the `!` prefix takes no keyboard input. It also sets up Apple's Command Line Tools, which can take a while. Then use `/opt/homebrew/bin/brew` directly and confirm with `brew --version`, since the running session may not see the new `PATH`. If they decline, stop before touching their files and explain what's missing.

   > **Correction, 2026-11-05:** pasting the brew.sh command no longer works for this audience: macOS 26.6.2 answers with "Possible Malware, Paste Blocked". The agent should open Homebrew's signed `.pkg` instead. See [the VM run](2026-11-05-vm-run-findings.md).

2. **Tools.** `agents`, `job`, `contextfill`, `sleepy`, `peep` and `woodcase`.
3. **Git.** Check for it after step 1. On a Mac without the Command Line Tools, `/usr/bin/git` is a stub, so `command -v git` passes and running it opens Apple's install dialog. Check for `/Library/Developer/CommandLineTools/usr/bin/git`, the same file Homebrew checks.

   > **Correction, 2026-11-05.** This first said to use `xcode-select -p`. That was wrong for this purpose: it succeeds on a Mac with only Xcode, where Homebrew would still install the tools. See [the research](2026-11-05-clean-mac-research.md).

   `git init`, `.gitignore` and a first commit, each with consent. Create no remote and install no hooks.
4. **`gh`.** Detect it and suggest it if missing, but never get stuck on it, because local Git works without an account. If they say yes, explain what Git and GitHub are and why we use them, then walk them through an account and `gh auth login` in a real terminal.
5. **Project.** Detect the stack, or ask what the project is if the folder is empty. Enable the default modules plus what's detected (`go`, `swift`, `swiftui`, `python`, `web`). Ask about what can't be detected: `stage-pilot` or `stage-build`, and whether to add `images`. Run `agents init`; if it stops on a `CLAUDE.md` of the project's own, run `agents migrate`, then `agents init` again. Create `local/` and its `.gitignore` entry. Unless `.jobs.db` already exists, run `job init --as claude` (or `--as opencode`), then `job gitignore`. See [the init findings](2026-11-05-init-existing-project-findings.md).
6. **Install path.** If the project will install something, offer to add the default path to `PATH`: `~/go/bin` for Go and `~/.swiftpm/bin` for Swift.
7. **Machine settings.** Offer each one separately (below).

## Decisions

All from Ben, 2026-11-05.

- **Formulae, not casks.** The tap already signs and notarizes bottles, so casks would be a second pipeline for no gain.
- **People pin Ben's `agents-md`** and move with `agents update`. Their `rule:` feedback doesn't reach him, and that's accepted.
- **`harness` is a default module** (`agents-md` commit b6e1a33, pushed since this was written). `delegation.md` links to it, and an `agents init` scratch run showed the file missing before the change and present after.
- **The main-thread context line is 33%.** The setup installs `contextfill hook --max 40 --main-max 33`.
- **The `local/` rule lives in this repo**, not in `agents-md`.
- **Machine-wide settings only with consent:** the `contextfill` hook and the plan-mode block. Not the iTerm status hook or permission settings.
- **Claude Code and OpenCode share one playbook**, with a short section per tool and any gaps documented.
- **`cappy` and `timetattle` are out for now.** See [the backlog](backlog.md).
- **The Bash sandbox is left off by default.** `harness.md` should say its sandbox failures only apply when it's on.
- **Ben-specific wording comes out of the shared modules**, so the rules read correctly on anyone's machine. The change reaches all of Ben's repos on update.
- **The public repo is `bensyverson/bootstrap`**, created and pushed when ready. Ben has approved that.
- **`contextfill` stays pure.** It measures the fill and compares it to the number it's given: no config file, no built-in limit, no advice. The hook command or plugin holds a person's numbers as flags, and the rules say what the line means. The rules keep every sentence and all their numbers, call 33% and 40% defaults, and add that a line named by the hook wins. They also say wrap-up starts at the line, it isn't a cutoff, and to keep working until then.

## Tasks

Imported into `job` on 2026-11-05 as root `SddU2d`; the tracker is the current version.

```yaml
tasks:
  - title: Set up a machine and project from one pasted link
    children:
      - title: Decide whether to push agents-md and move the 33% limit everywhere
        ref: publish
        labels: [decision]
        desc: |
          New users get `agents-md` from GitHub, so the `harness` default and the 33% limit don't reach them until it's pushed. `main` is two commits ahead: 22b9a51 (Ben's, raises the limit to 33%) and b6e1a33 (adds `harness`). The repo is public, so pushing needs Ben's OK. His uncommitted `modules/design-process.md` edit stays out of it.

          After the push, run `agents update` in this repo, which is still pinned to 99458a4 and renders 25%. `contextfill`'s README and OpenCode plugin both still say `--main-max 25`, and so does Ben's own global hook, which is his to change.

          Out of scope: rolling the new pin across Ben's other repos.
        criteria:
          - Ben has ruled on the push and any push is done
          - This repo renders 33% after agents update
          - The contextfill README and plugin say --main-max 33, or a note says why not
      - title: Find out what agents init and job init do to an existing project
        ref: probe
        desc: |
          The setup has to be safe in a project that already has files, and `SETUP.md` should be written from what these tools actually do. The `agents init` help says existing `AGENTS.md` content becomes the project-owned head and that it links `CLAUDE.md`. No one has yet checked a real `CLAUDE.md` file with no `AGENTS.md`, or a symlink, an existing `.gitignore`, a folder that isn't a Git repo, or a second run.

          Try each case in a scratch folder, with `AGENTS_REGISTRY` and `AGENTS_CACHE` pointed at scratch so Ben's real registry stays clean. Fix a tool bug at its source, or file it, rather than working around it in the playbook.

          Where the answer goes: a dated findings doc under `project/`.
        criteria:
          - A findings doc lists each case, the command run and the outcome
          - Any case that loses user content is fixed or has a job filed against it
      - title: Ship the Go tools as signed bottles
        ref: go-tools
        desc: |
          `job` and `agents` only install with `go install`, and `contextfill` builds from source, so a new user would need Go. Add formulae for `job` and `agents` to `homebrew-tap`, and set `bottle=yes`, `sign=` and `notary_profile=` in each tool's `scripts/release.conf`, following `woodcase`'s. Each tool releases itself with `homebrew-tap`'s `scripts/release`.

          Check each repo is public first; `gh` was blocked in the sandbox when this plan was written. Releases push tags and upload bottles, so they need Ben's OK and his signing credentials. Update each README's install section to lead with `brew install bensyverson/tap/<tool>`.

          Risk: bottles are tagged for the formula's oldest macOS, and the binary must target that release.
        criteria:
          - brew install of job, agents and contextfill pours a bottle with no Go installed, shown on a clean account
          - codesign and spctl checks pass on each binary
      - title: Ship sleepy and peep as signed bottles
        ref: swift-tools
        desc: |
          Neither tool is in the tap: `sleepy` (repo `sleepyhollow`) and `peep` (repo `PixelPeeper`) install from source today. Add formulae and bottles the same way `woodcase` does, including any resource bundle that has to sit beside the binary.

          Two traps. `sleepy`'s `Package.swift` deliberately pins the macOS 12 SDK at link time because WebKit gates behavior on it, so a build that loses that flag can pass and still behave wrongly. `PixelPeeper` has no LICENSE file, and Ben needs to confirm one before it ships publicly.

          Same release constraints as the Go tools: Ben's OK and his signing credentials.
        criteria:
          - brew install of sleepy and peep pours a bottle without Xcode, shown on a clean account
          - otool -l on the bottled sleepy shows the same SDK version as a source build
          - PixelPeeper has a LICENSE
      - title: Write the README, SETUP.md and Brewfile
        ref: playbook
        blockedBy: [probe]
        desc: |
          This is the product. The plan doc's "How the link works" and "Setup order" sections hold the agreed flow, including the Homebrew, `gh` and `PATH` steps, and its "Decisions" section binds this leaf.

          Write for someone who may never have used a terminal: explain each tool in a sentence or two. Keep the README tight and put the steps in `SETUP.md`. A fetch tool may summarize long pages, so tell the agent to read the raw file and say how. Each step checks before it acts, so a second run changes nothing. Include a short section per tool for Claude Code and OpenCode, and document what OpenCode lacks: `harness.md` only covers Claude Code.

          `SETUP.md` also creates `local/` and explains it, because that rule lives here. Installing the tools depends on the bottle leaves for real use, but the text can be written before they ship.
        criteria:
          - The README opens with what this is and what changes, and points agents at SETUP.md
          - SETUP.md covers every step in the plan, including declining Homebrew and declining gh
          - A dry read in a fresh session finds no step that needs this planning conversation
      - title: Ship the machine-wide settings
        ref: settings
        desc: |
          Two Claude Code settings are recommended, each offered with consent: the `contextfill` hook (`--max 40 --main-max 33`) and the plan-mode block. Ben's block lives at `~/.claude/hooks/block-plan-mode.sh`, outside any repo, and it needs `jq`. Decide where the script ships from and add `jq` if it stays a dependency. It only denies plan mode in folders with a `.jobs.db`, so it's safe to install globally.

          The setup must merge into an existing `settings.json`, never overwrite it. OpenCode gets the `contextfill` plugin link from the formula's caveats.
        criteria:
          - Running the step on an existing settings.json keeps every existing key, shown by a before and after diff
          - A second run adds nothing
          - The hook reports fill in a live session on a clean account
      - title: Decide whether the setup recommends the Bash sandbox
        labels: [decision]
        desc: |
          `harness.md` assumes the Bash sandbox is on, so a new user without it reads rules about failures they won't see. Options: recommend it as a third setting, or leave it off and reword `harness.md`. Ben's question to answer; the settings and playbook leaves wait on it only for one paragraph.
      - title: Decide whether to remove Ben-specific wording from the modules
        labels: [decision]
        desc: |
          Some shared rules assume Ben's machine: `web.md` says `sleepy` is "on this machine". Rewording reaches all his repos on their next update, and the question is still open with Ben.
      - title: Run the whole setup on a clean Mac
        blockedBy: [publish, go-tools, swift-tools, playbook, settings]
        desc: |
          This is the plan's definition of done. Ben's own Mac already has everything installed, so only a new macOS account or VM counts, starting without Homebrew or the Command Line Tools. Paste the repo URL with no other words into fresh Claude Code and OpenCode sessions, once in an empty folder and once in an existing project, then run it a second time.

          Needs Ben or another person at the keyboard for the terminal steps.
        criteria:
          - An empty folder and an existing project both finish with a clean agents check and a working job orient, in Claude Code
          - The same two runs finish in OpenCode, with any gaps matching what SETUP.md documents
          - A second run changes nothing, shown by git status
          - Declining Homebrew leaves the folder untouched
          - Findings are recorded in a dated doc under project/
```
