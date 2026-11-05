# What's stock on macOS

**Finding:** a script could run on the system Perl or Ruby before the developer tools are installed. Python can't do this. Apple warned in 2019 that these runtimes will eventually go, and no source found says when. We ended up choosing a Go binary instead, for the reasons under Decision.

## On this Mac

macOS 27.0.1, with Xcode installed, so this shows what's in the protected system volume rather than a run on a new machine. The clean-Mac test settles it.

Linked to the developer-tools switcher means `otool -L /usr/bin/<name>` lists `xcselect`, so the binary hands off to the developer tools rather than running by itself.

| Name | In `/usr/bin` | Linked to the switcher | Usable before the developer tools |
|---|---|---|---|
| `perl` | Yes (5.34.1) | No | Yes |
| `ruby` | Yes (2.6.10) | No | Yes |
| `tclsh`, `awk`, `sqlite3`, `nc`, `curl`, `osascript` | Yes | No | Yes |
| `python3` | Yes | Yes | No, by inference |
| `git`, `make`, `clang`, `swift` | Yes | Yes | No, by inference |
| `python`, `python2`, `php`, `node`, `lua` | No | n/a | No |

The "by inference" rows weren't run without the tools. Published accounts of Python on Tahoe fit: it comes with the developer tools as an internal dependency.

Server modules load from the system volume, shown with `perl -e 'print $INC{...}'` and Ruby's `$LOADED_FEATURES`:

- `HTTP::Daemon`: `/System/Library/Perl/Extras/5.34/HTTP/Daemon.pm`
- `IO::Socket::INET`: `/System/Library/Perl/5.34/darwin-thread-multi-2level/IO/Socket/INET.pm`
- `webrick`: `/System/Library/Frameworks/Ruby.framework/Versions/2.6/usr/lib/ruby/2.6.0/webrick.rb`

## What the web says

- **Apple's direction:** the Catalina notes, as quoted by others, say future versions won't include scripting runtimes by default. I didn't open those notes myself.
- **Python:** removed from the base system in macOS 12.3. Sources disagree on whether Apple's notes said so.
- **Ruby and Perl on 27:** I found no announcement of removal. Apple's macOS 27 release notes loaded with only a title, so they're unchecked. [ComputingForGeeks](https://computingforgeeks.com/macos-golden-gate-for-developers/) lists both as shipping, but its output appears to come from macOS 26.
- **Apple on Ruby:** in December 2025, an Apple engineer [said](https://developer.apple.com/forums/thread/808618) "Ruby is part of the macOS base system" and that changing this is "hard to do without breaking folks." This is about Tahoe and isn't a promise.
- **A false lead:** [NIST's "no more ruby" pull request](https://github.com/usnistgov/macos_security/pull/783) drops that project's own Ruby dependency. It says nothing about macOS.
- **MacBook Neo:** [Apple's specs](https://www.apple.com/macbook-neo/specs/) list an A18 Pro and 8GB of memory. They name no macOS version, so this doesn't confirm which release a new Neo ships with.

## Decision

> **Superseded the same day.** We first chose the system Perl with `HTTP::Daemon`. Ben then chose a signed Go binary, because the page grew to need live progress, several embedded assets and a heartbeat. The runtime facts above still stand. See [the plan](2026-11-05-setup-link-plan.md).

If the binary can't start, the agent carries on in the chat.
