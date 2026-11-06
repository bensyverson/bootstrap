#!/bin/sh
# vm-snap — print what the setup can change on a Mac, one line per file, so two
# snapshots diff to exactly what a run changed. Runs on the Mac being checked (a
# trial VM), with nothing but what macOS ships: never git or python3, which are
# Apple's stand-ins on a clean Mac and pop the developer tools dialog.
#
# Usage, from grace (the VM's address from `tart ip <vm>`):
#     scripts/vm-snap.sh --remote <ip> > before.txt
#     … run the thing …
#     scripts/vm-snap.sh --remote <ip> > after.txt && diff before.txt after.txt
# --remote sends this script to the VM through the Mini's vm.exp and prints its output.
set -eu

if [ "${1:-}" = --remote ]; then
    body=$(base64 <"$0" | tr -d '\n')
    ssh agent@sally.local "~/tart-install/vm.exp \"echo $body | base64 -D | sh\" $2" |
        tr -d '\r' | sed -n '/^@@SNAP@@$/,/^@@END@@$/p' | sed '1d;$d'
    exit 0
fi

echo @@SNAP@@
for f in ~/.gitconfig ~/.zshrc ~/.claude/settings.json ~/.bootstrap/preferences.json ~/.config/agents/repos; do
    [ -f "$f" ] && echo "file $f $(shasum -a 256 "$f" | cut -d' ' -f1)"
done
for d in ~/Projects ~/.config/opencode ~/.bootstrap/bin; do
    [ -d "$d" ] && find "$d" -type f -not -path '*/.git/objects/*' | sort | while IFS= read -r f; do
        echo "file $f $(shasum -a 256 "$f" | cut -d' ' -f1)"
    done
done
ls /opt/homebrew/Cellar 2>/dev/null | while IFS= read -r f; do
    echo "cellar $f $(ls /opt/homebrew/Cellar/"$f" | tr '\n' ' ')"
done
echo @@END@@
