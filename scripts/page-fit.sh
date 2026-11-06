#!/bin/bash
# Measures, at each window size, how far every preview state's words stay above the progress percentage
# ("clear", in px) and where they start ("top"); flags any state closer than 12px or starting above the window.
# usage: scripts/page-fit.sh <preview-base-url-with-token> [agent-suffix] [WxH ...]
#   e.g. scripts/page-fit.sh 'http://127.0.0.1:50000/preview?token=abc' .claude-code 1024x640 1440x900
# Defaults: .claude-code at 1024x640 (the trial VM's Safari), 1280x800 and 1440x900.
# Needs sleepy on PATH and a running `bootstrap serve` (BOOTSTRAP_OPEN=/usr/bin/true keeps the browser shut).
set -u
base=$1 only=${2:-.claude-code}
shift $(($# < 2 ? $# : 2))
sizes=("$@")
[ ${#sizes[@]} -eq 0 ] && sizes=(1024x640 1280x800 1440x900)
here=$(dirname "$0")
bad=0
for size in "${sizes[@]}"; do
  json=$(sleepy eval "$base#welcome$only" --size "$size" --wait-for 'html[data-drawn]' --budget 120000 \
    --args "{\"only\":\"$only\"}" --file "$here/page-fit.js") || { echo "PROBE FAILED $size"; bad=$((bad + 1)); continue; }
  report=$(printf '%s' "$json" | SIZE=$size python3 -I -c '
import json, os, sys
size, worst = os.environ["SIZE"], 0
for s in json.load(sys.stdin):
    flag = "  OVERLAP" if s["clear"] < 12 else ("  OFF THE TOP" if s["top"] < 0 else "")
    worst += bool(flag)
    print("%s %-44s clear %5d  top %5d%s" % (size, s["name"], s["clear"], s["top"], flag))
print(f"{size}: {worst} problems")
')
  echo "$report"
  case "$report" in *": 0 problems") ;; *) bad=$((bad + 1)) ;; esac
done
[ "$bad" = 0 ]
