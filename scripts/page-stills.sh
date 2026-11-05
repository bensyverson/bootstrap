#!/bin/bash
# Draws every preview state with Sleepy: a still, console output, overflow at 400px and the request log.
# usage: scripts/page-stills.sh <preview-base-url-with-token> <out-dir> [state-name-filter]
#   e.g. scripts/page-stills.sh 'http://127.0.0.1:50000/preview?token=abc' /tmp/stills tool
# Needs sleepy on PATH and a running `bootstrap serve`. Headless Sleepy never ticks animation frames,
# so this checks stills at rest and says nothing about motion.
set -u
base=$1 out=$2 filter=${3:-}
mkdir -p "$out"
origin=${base%%/preview*}
query=${base#*\?}
names=$(curl -s "$origin/preview.json?$query" | python3 -I -c 'import json,sys; print("\n".join(p["name"] for p in json.load(sys.stdin)))')
bad=0 count=0
for name in $names; do
  case "$name" in *"$filter"*) ;; *) continue ;; esac
  count=$((count + 1))
  url="$base#$name"
  sleepy shot "$url" --size 1440x900 --wait-for 'html[data-drawn]' --out "$out/$name.png" >/dev/null || { echo "SHOT FAILED $name"; bad=$((bad + 1)); }
  sleepy shot "$url" --size 400x800 --wait-for 'html[data-drawn]' --out "$out/$name.400.png" >/dev/null || { echo "SHOT FAILED 400 $name"; bad=$((bad + 1)); }
  over=$(sleepy overflow "$url" --size 400x800 --wait-for 'html[data-drawn]' --format text 2>&1) || { echo "OVERFLOW $name: $over"; bad=$((bad + 1)); }
  sleepy console "$url" --size 1440x900 --wait-for 'html[data-drawn]' --format text > "$out/$name.console.txt" 2>&1
  [ -s "$out/$name.console.txt" ] && grep -qv '^$' "$out/$name.console.txt" && { echo "CONSOLE $name:"; cat "$out/$name.console.txt"; bad=$((bad + 1)); }
done
echo "checked $count states, $bad problems"
[ "$bad" = 0 ]
