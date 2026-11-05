# Setting up a Mac with bootstrap

> **This is a trial.** It goes as far as the welcome and list screens, then stops. It installs nothing: no Homebrew, no tools, no changes to any project. The full setup comes later.

You are a coding agent, and the person you're working with pasted a link to [bensyverson/bootstrap](https://github.com/bensyverson/bootstrap). This file is your playbook. Read it raw, since a summary can drop a step: `https://raw.githubusercontent.com/bensyverson/bootstrap/main/SETUP.md`.

## What this is

`bootstrap` is a small program by Ben Syverson, signed with his Apple Developer ID and notarized by Apple. It opens a page in the person's browser that shows each step and collects their choices. You run it and drive it; it never installs anything itself.

## What will change on this Mac

Tell the person this, in plain words, and wait for their OK before you do anything:

- One file is downloaded: `~/.bootstrap/bin/bootstrap`.
- A private folder, `~/.bootstrap/`, holds that file and a log of this session.
- A page opens in their browser. It's served from this Mac only (`127.0.0.1`), and nothing on it is sent anywhere.
- Nothing else changes: no installs, no settings, no files in this folder.

To undo it all later: `~/.bootstrap/bin/bootstrap stop`, then `rm -rf ~/.bootstrap`.

## 1. Check this Mac

```sh
uname -m
sw_vers -productVersion
```

`bootstrap` needs Apple silicon (`arm64`) and macOS 15 or later. If either doesn't hold, say so in one sentence and stop: this trial has nothing for that Mac.

## 2. Fetch the pinned file

This version and its checksum are pinned here on purpose. Don't swap in a newer release.

```sh
mkdir -p ~/.bootstrap/bin
curl -fsSL -o ~/.bootstrap/bin/bootstrap.download https://github.com/bensyverson/bootstrap/releases/download/v0.1.0/bootstrap
```

## 3. Verify it before running it

You can't read a binary, so these checks are how you know it's the file Ben released.

```sh
echo "53f64c0ef0b131bf54fdad1108acbbfaee780766daa8c0039d376ce7589294ab  $HOME/.bootstrap/bin/bootstrap.download" | shasum -a 256 -c
codesign --verify --strict ~/.bootstrap/bin/bootstrap.download
codesign -dv ~/.bootstrap/bin/bootstrap.download 2>&1 | grep TeamIdentifier
spctl -a -vvv -t open --context context:primary-signature ~/.bootstrap/bin/bootstrap.download
```

Stay in the person's folder while you do this; `bootstrap` looks at the folder it runs from.

- **The checksum must say `OK`.** If it doesn't, delete the file, tell the person, and stop.
- **The signature must verify, and `TeamIdentifier` must be `QJ3Q58XVEE`** (Syverson Industries LLC). If not, delete the file, tell the person, and stop.
- **Gatekeeper (`spctl`) should say `accepted` and `source=Notarized Developer ID`.** Use the command as written: the more common `spctl -a -t exec` rejects every command-line program, because it only accepts apps. Gatekeeper checks with Apple over the network, and it has rejected a good file before. So if the checksum and signature passed, a `spctl` failure alone isn't a reason to stop: tell the person what it said, and go on.

Then make it runnable:

```sh
mv ~/.bootstrap/bin/bootstrap.download ~/.bootstrap/bin/bootstrap
chmod +x ~/.bootstrap/bin/bootstrap
```

## 4. Start it

Call it by its full path, since it isn't on `PATH`:

```sh
~/.bootstrap/bin/bootstrap status
```

From here, follow the `next:` lines it prints; they name the command to run each time. Running `bootstrap` alone explains every command, and any command takes `--help`. When you start the page, pass the app you are: `--agent claude-code` or `--agent opencode`.

In this trial, show the `welcome` screen, wait for the answer, then show the `list` screen and wait again. Once the person has answered the list, tell them the trial ends here and nothing was installed, and run `~/.bootstrap/bin/bootstrap stop`.

If the page can't open, the program says so. Tell the person in one sentence, and stop.
