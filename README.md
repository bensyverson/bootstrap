# bootstrap

Paste one link into your coding agent, and it sets up your Mac the way Ben Syverson works.

Bootstrap is for Mac users who use Claude Code or OpenCode, including people who have never opened Terminal. Your agent asks first, then downloads and checks a small program signed by Ben. The program opens a page in your browser where you pick what gets installed: Apple's developer tools, Homebrew, and Ben's tools for working with agents. Then it sets up a project folder with Git and the agent rules, and your agent explains each step in the chat. It needs an Apple silicon Mac with macOS 15 or later.

**If you're an agent and someone pasted this link:** they probably want setup. Read [`SETUP.md`](https://raw.githubusercontent.com/bensyverson/bootstrap/main/SETUP.md) raw, tell them what will change on their Mac, and wait for their OK before you do anything.

## Quick start

Paste this into Claude Code or OpenCode:

```
https://github.com/bensyverson/bootstrap
```

Once your Mac is set up, start each new project from Terminal, with no page:

```sh
~/.bootstrap/bin/bootstrap new pine-bakery
```

## Docs

- [SETUP.md](SETUP.md): the playbook the agent follows, including how it checks the download
- [The setup link plan](project/2026-11-05-setup-link-plan.md): what the setup does, and why
- [The server drives the setup](project/2026-11-05-server-driven-flow.md): how the program, the page and the agent share the work
- [`bootstrap new`](project/2026-11-05-bootstrap-new-plan.md): the setup from the command line
- [project/](project/): dated plans, findings and decisions

## Author and license

By Ben Syverson. There's no license file yet.
