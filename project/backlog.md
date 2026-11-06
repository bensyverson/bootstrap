# Backlog

Work decided against, parked rather than dropped in silence. Nothing here is scheduled or blocking — active work lives in `job`. Read it before proposing something that sounds novel; it may already have been weighed and parked.

- **One dated H2 per item:** what it is, why it's parked, and *what would un-park it*.
- **Delete an item when it lands, or when it stops being plausible.** A long list is one nobody reads.
- **Something parked that turns out to be needed becomes a task in `job`** — move it, don't work it from here.

Format: one dated H2 per entry, a headline, then what it is, why it's parked, and the trigger that would revive it.

---

## 2026-11-06: A chat-only setup

SETUP.md used to carry the whole setup a second time, in prose (13 steps, ~95 lines), for when the `bootstrap` program couldn't run, with `TestTheChatOnlySetupFollowsTheFlowsSteps` keeping its step order in line with `internal/flow`. Ben removed it (2026-11-06) before its first real run (`W2yASt`'s `gjk`): on a clean Mac, nearly every reason the program can't run also stops the prose copy. A blocked download blocks Homebrew's `.pkg` too, since both come from GitHub's release-assets host; an older macOS ends the setup either way; the sandbox and a model that won't loop break both. The program's own `resume --retry`/`--skip` gets past a broken step. SETUP.md now tells the person what stays, how to pick up later, and where to report it. The last version is `git show ab39cfe:SETUP.md`, section "The chat-only setup".

**Would un-park it:** a real report of someone stuck because the program itself was broken where an agent could have finished by hand, or a common case where people say yes to the setup but no to running a downloaded program.

## 2026-11-05: Installing `timetattle` during setup

The setup would install `timetattle` for new users. It's parked because it is shaped for Ben's own use and runs constantly, which asks more of someone's machine than the other tools do, and nothing in the setup depends on it.

Un-park it if someone who has been through the setup asks how Ben tracks time, or if it gains a form that suits other people, such as running only when asked.

## 2026-11-05: Installing `cappy` during setup

`cappy` belongs in the supporting tools, but its README says it is planned and not yet built, so there is nothing to install. Ben ruled it can wait.

Un-park it when `cappy` has a first release in the tap.

## 2026-11-05: A `say` verb for the agent's own words on the page

`bootstrap say` would let the agent put free text on the setup page. It's parked because the page's words are closed lists on purpose: they stay short and simple, and an agent's guess would read as a fact from the setup itself. The chat is already where the agent can say anything, and the "Let's talk in {app}" screen sends people there. Everything happening in the main session is Ben's ruling.

Un-park it if the VM run or the agent trial keeps hitting moments where the page is too vague to be useful and a new reviewed screen or status wouldn't fix it. Try showing data the binary can check first, like elapsed time. **If it's ever built (Ben, 2026-11-05),** it must appear as a message clearly from Claude or OpenCode, never as a normal title.
