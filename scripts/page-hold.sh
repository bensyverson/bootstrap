#!/bin/bash
# Checks that a working screen replaced within the page's hold never draws, and that every other screen
# still does: walks the preview page on a clock with Sleepy and records each panel that draws.
# usage: scripts/page-hold.sh <preview-base-url-with-token>
#   e.g. scripts/page-hold.sh 'http://127.0.0.1:50000/preview?token=abc'
# Needs sleepy on PATH and a running `bootstrap serve` (BOOTSTRAP_OPEN=/usr/bin/true keeps the browser shut).
# It checks which screens draw, not how they move: headless Sleepy never ticks animation frames.
set -u
base=$1
here=$(dirname "$0")
result=$(sleepy eval "$base#welcome.claude-code" --size 1440x900 --wait-for 'html[data-drawn]' --budget 30000 --file "$here/page-hold.js")
echo "$result"
case "$result" in *'"ok":true'* | *'"ok" : true'*) ;; *) exit 1 ;; esac
