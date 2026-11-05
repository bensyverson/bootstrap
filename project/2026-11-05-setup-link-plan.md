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

An agent may rightly treat a page's instructions as untrusted. Stating the changes and asking first is the design, not a workaround. How agents treat a bare link is untested until the acceptance leaf.

## Setup order

Each step skips what's already done, so running it again is safe. Every install says what the tool is and why it's being installed before it happens.

0. **Where are we?** On a new Mac, Claude Code may well be running in the home folder. Never run `git init` or `agents init` there: offer to create a project folder and have the user restart the session in it.
1. **Homebrew.** If it's missing, explain it and offer a walkthrough. The user runs the official brew.sh command in a real terminal, because it can ask for their Mac password and the `!` prefix takes no keyboard input. It also sets up Apple's Command Line Tools, which can take a while. Then use `/opt/homebrew/bin/brew` directly and confirm with `brew --version`, since the running session may not see the new `PATH`. If they decline, stop before touching their files and explain what's missing.
2. **Tools.** `agents`, `job`, `contextfill`, `sleepy`, `peep` and `woodcase`.
3. **Git.** Check for it after step 1. On a Mac without the Command Line Tools, `/usr/bin/git` is a stub, so `command -v git` passes and running it opens Apple's install dialog; use `xcode-select -p` to check. `git init`, `.gitignore` and a first commit, each with consent. Create no remote and install no hooks.
4. **`gh`.** Detect it and suggest it if missing, but never get stuck on it, because local Git works without an account. If they say yes, explain what Git and GitHub are and why we use them, then walk them through an account and `gh auth login` in a real terminal.
5. **Project.** Detect the stack, or ask what the project is if the folder is empty. Enable the default modules plus what's detected (`go`, `swift`, `swiftui`, `python`, `web`). Ask about what can't be detected: `stage-pilot` or `stage-build`, and whether to add `images`. Create `local/` and its `.gitignore` entry, and run `job init` and `job gitignore`.
6. **Install path.** If the project will install something, offer to add the default path to `PATH`: `~/go/bin` for Go and `~/.swiftpm/bin` for Swift.
7. **Machine settings.** Offer each one separately (below).

## Decisions

All from Ben, 2026-11-05.

- **Formulae, not casks.** The tap already signs and notarizes bottles, so casks would be a second pipeline for no gain.
- **People pin Ben's `agents-md`** and move with `agents update`. Their `rule:` feedback doesn't reach him, and that's accepted.
- **`harness` is a default module** (`agents-md` commit b6e1a33, unpushed). `delegation.md` links to it, and an `agents init` scratch run showed the file missing before the change and present after.
- **The main-thread context line is 33%.** The setup installs `contextfill hook --max 40 --main-max 33`.
- **The `local/` rule lives in this repo**, not in `agents-md`.
- **Machine-wide settings only with consent:** the `contextfill` hook and the plan-mode block. Not the iTerm status hook or permission settings.
- **Claude Code and OpenCode share one playbook**, with a short section per tool and any gaps documented.
- **`cappy` and `timetattle` are out for now.** See [the backlog](backlog.md).
- **The Bash sandbox is left off by default.** `harness.md` should say its sandbox failures only apply when it's on.
- **Ben-specific wording comes out of the shared modules**, so the rules read correctly on anyone's machine. The change reaches all of Ben's repos on update.
- **The public repo is `bensyverson/bootstrap`**, created and pushed when ready. Ben has approved that.
- **`contextfill` defaults to a 33% main-thread limit**, and people should be able to configure it. How is still open.

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
