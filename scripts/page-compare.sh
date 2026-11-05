#!/bin/bash
# Compares preview stills with the approved prototype's, by Mean Absolute Error from `peep compare`.
# usage: scripts/page-compare.sh <preview-base-url-with-token> <prototype-local.html> <out-dir>
# Build the prototype's local.html with its build.py first, in a copy so the mock stays untouched.
# Both stills stop 80px above the bottom, which leaves out the prototype's study bar.
set -u
base=$1 proto=$2 out=$3
mkdir -p "$out"
# preview state : prototype hash
pairs="
welcome.claude-code:welcome
welcome.opencode:welcome-opencode
list.claude-code:list
fast.claude-code:fast
apple-ask.claude-code:appleAsk
apple-wait.claude-code:appleWait
terminal.claude-code:terminal
brew-wait.claude-code:brewing
tool.agents.downloading.claude-code:tools-agents
tool.jobs.downloading.claude-code:tools-jobs
tool.contextfill.downloading.claude-code:tools-contextfill
tool.sleepy.downloading.claude-code:tools-sleepy
tool.peep.downloading.claude-code:tools-peep
tool.woodcase.downloading.claude-code:tools-woodcase
making.claude-code:making
xcode.claude-code:xcode
live-yet.claude-code:liveYet
naming.claude-code:naming
building.folder.claude-code:building
github.claude-code:github
phone.claude-code:phone
done.claude-code:done
trouble.claude-code:trouble
fixing.claude-code:fixing
lost.claude-code:lost
"
for size in 1440x900:820 400x800:720; do
  dims=${size%%:*} height=${size##*:} width=${dims%%x*}
  for pair in $pairs; do
    name=${pair%%:*} hash=${pair##*:}
    a="$out/$name.$width.port.png" b="$out/$name.$width.proto.png"
    sleepy shot "$base#$name" --size "$dims" --wait-for 'html[data-drawn]' --rect "0,0,$width,$height" --out "$a" >/dev/null
    sleepy shot "file://$proto#$hash" --size "$dims" --rect "0,0,$width,$height" --out "$b" >/dev/null
    echo "$width $name $(peep compare "$a" "$b" | head -1)"
  done
done
