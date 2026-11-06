#!/usr/bin/env bash
# page-demo — run the real setup page against a pretend Mac, to judge its motion live.
#
# Usage: scripts/page-demo.sh [--agent claude-code|opencode] [--delay <seconds>] [<binary>]
#
# Makes a throwaway root and home under $TMPDIR holding Apple's tools, Homebrew, a
# GitHub sign-in and most of the tap's tools as stand-in programs, then starts
# `bootstrap run` there with your browser. Nothing on this Mac is changed: every
# command the flow runs is the stand-in under the root, and its files land in the
# throwaway home. sleepy and bootstrap's own formula are missing, so the tools step
# installs one tool and quietly keeps the downloaded bootstrap. Each stand-in brew
# call waits --delay seconds (default 0), so 0 shows the quickest steps, which is
# where a working screen could flash.
#
# The binary defaults to local/bs, built from this checkout. Stop with:
#     HOME=<the home it prints> <binary> stop
# Needs the Bash sandbox off: the page binds a port and opens your browser.
set -euo pipefail

agent=claude-code
delay=0
bin=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --agent) agent="$2"; shift 2 ;;
        --delay) delay="$2"; shift 2 ;;
        -h|--help) sed -n '2,/^set -euo/p' "$0" | sed '$d' | sed 's/^# \{0,1\}//'; exit 0 ;;
        -*) echo "page-demo: unknown flag $1" >&2; exit 2 ;;
        *) bin="$1"; shift ;;
    esac
done
repo="$(cd "$(dirname "$0")/.." && pwd -P)"
if [[ -z "$bin" ]]; then
    bin="${repo}/local/bs"
    (cd "$repo" && go build -o "$bin" ./cmd/bootstrap)
fi

work="$(cd "$(mktemp -d "${TMPDIR:-/tmp}/bootstrap-demo.XXXXXX")" && pwd -P)"
root="${work}/root" home="${work}/home"
mkdir -p "$root" "${home}/pine-bakery"
calls="${work}/calls.log"

# stub <path under root> <body>: an executable stand-in.
stub() {
    mkdir -p "$(dirname "${root}/$1")"
    printf '#!/bin/sh\necho "%s $*" >> %q\n%s\n' "$(basename "$1")" "$calls" "$2" >"${root}/$1"
    chmod +x "${root}/$1"
}

stub Library/Developer/CommandLineTools/usr/bin/git "config=$(printf %q "${work}/gitconfig")
case \"\$1 \$2\" in
\"config --global\")
  if [ \"\$3\" = --get ]; then v=\$(grep \"^\$4=\" \"\$config\" 2>/dev/null | tail -1 | cut -d= -f2-); [ -n \"\$v\" ] || exit 1; echo \"\$v\"; exit 0; fi
  echo \"\$3=\$4\" >> \"\$config\"; exit 0 ;;
esac
[ \"\$1\" = init ] && { mkdir -p .git; exit 0; }
exit 0"
stub opt/homebrew/bin/brew "sleep ${delay}
case \"\$1\" in
trust) exit 0 ;;
install)
  f=\${2##*/}
  [ \"\$f\" = bootstrap ] && { echo 'Error: No available formula with the name \"bensyverson/tap/bootstrap\".'; exit 1; }
  printf '#!/bin/sh\\nexit 0\\n' > $(printf %q "$root")/opt/homebrew/bin/\$f; chmod +x $(printf %q "$root")/opt/homebrew/bin/\$f
  echo \"🍺  /opt/homebrew/Cellar/\$f/0.2.0: 5 files, 9MB\"; exit 0 ;;
upgrade) echo \"Warning: \${2##*/} 0.2.0 already installed\"; exit 0 ;;
esac
exit 1"
stub opt/homebrew/bin/gh 'case "$1 $2" in
"auth status") echo "✓ Logged in to github.com account ada"; exit 0 ;;
"api user") echo "{\"login\":\"ada\",\"id\":1234,\"name\":\"Ada Lovelace\"}"; exit 0 ;;
esac
exit 0'
stub opt/homebrew/bin/agents 'case "$1" in
init) printf "# rules\n" > AGENTS.md ;;
esac
exit 0'
stub opt/homebrew/bin/job 'case "$1" in
init) touch .jobs.db ;;
gitignore) echo ".jobs.db" >> .gitignore ;;
esac
exit 0'
for tool in contextfill peep woodcase; do
    stub "opt/homebrew/bin/${tool}" 'exit 0'
done

echo "pretend Mac: root=${root}"
echo "             home=${home}"
echo "             calls=${calls}"
cd "${home}/pine-bakery"
env -i PATH=/usr/bin:/bin HOME="$home" BOOTSTRAP_ROOT="$root" "$bin" run --agent "$agent"
echo "stop with: HOME=${home} ${bin} stop"
