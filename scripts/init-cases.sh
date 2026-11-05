#!/bin/bash
# Runs `agents init`, `job init` and `job gitignore` against folders that
# already hold files, and prints what each case looks like before and after.
# Feeds project/2026-11-05-init-existing-project-findings.md.
#
# Usage: scripts/init-cases.sh <scratch dir> [case ...]
# Needs github.com reachable (the agents-md source is fetched over HTTPS).
# The registry, cache and Git config all point into the scratch dir, so the
# real registry stays clean.
set -u

if [ $# -lt 1 ]; then
  echo "usage: $0 <scratch dir> [case ...]" >&2
  exit 2
fi
root=$1
shift
mkdir -p "$root"
root=$(cd "$root" && pwd)

export AGENTS_REGISTRY="$root/registry"
export AGENTS_CACHE="$root/cache"
export GIT_CONFIG_GLOBAL="$root/gitconfig"
export XDG_CONFIG_HOME="$root/xdg"
git config --file "$GIT_CONFIG_GLOBAL" user.name "Init Cases"
git config --file "$GIT_CONFIG_GLOBAL" user.email "init-cases@example.invalid"
git config --file "$GIT_CONFIG_GLOBAL" init.defaultBranch main

source_url=github.com/bensyverson/agents-md
agents_bin=${AGENTS_BIN:-/opt/homebrew/bin/agents}
job_bin=/opt/homebrew/bin/job

run() {
  echo "\$ $*"
  "$@"
  echo "[exit $?]"
}

show() {
  echo "--- files"
  (cd "$1" && find . -path ./.git -prune -o -print | sort | while read -r f; do
    if [ -L "$f" ]; then echo "$f -> $(readlink "$f")"; else echo "$f"; fi
  done)
  for f in CLAUDE.md .gitignore; do
    if [ -e "$1/$f" ]; then
      echo "--- $f"
      cat "$1/$f"
      echo "<EOF>"
    fi
  done
  if [ -e "$1/AGENTS.md" ]; then
    echo "--- AGENTS.md head (up to the first generated region)"
    sed '/<!-- agents:begin/q' "$1/AGENTS.md"
  fi
  if [ -d "$1/.git" ]; then
    echo "--- git status"
    git -C "$1" status --short
  fi
}

setup_all() {
  ( "$agents_bin" init "$source_url" ) 2>&1
  echo "[agents init exit $?]"
  ( "$job_bin" init --as claude ) 2>&1
  echo "[job init exit $?]"
  ( "$job_bin" gitignore ) 2>&1
  echo "[job gitignore exit $?]"
}

case_dir() {
  d="$root/cases/$1"
  rm -rf "$d"
  mkdir -p "$d"
  echo "$d"
}

# Each case_* builds its folder, then runs the setup inside it.
case_empty_nogit() {
  :
}

case_empty_git() {
  git init -q .
}

case_claude_only() {
  git init -q .
  printf '# My project\n\nAlways use tabs.\n' > CLAUDE.md
}

case_claude_symlink_out() {
  git init -q .
  mkdir -p ../shared-$1
  printf '# Shared rules\n\nUse tabs.\n' > ../shared-$1/rules.md
  ln -s ../shared-$1/rules.md CLAUDE.md
}

case_claude_symlink_agents() {
  git init -q .
  printf '# Mine\n\nUse tabs.\n' > AGENTS.md
  ln -s AGENTS.md CLAUDE.md
}

case_both_differ() {
  git init -q .
  printf '# Agents rules\n\nRun the tests.\n' > AGENTS.md
  printf '# Claude rules\n\nUse tabs.\n' > CLAUDE.md
}

case_gitignore_no_newline() {
  git init -q .
  printf 'node_modules/\n*.log' > .gitignore
}

case_gotchas_exists() {
  git init -q .
  mkdir project
  printf '# Our gotchas\n\n- Mine.\n' > project/gotchas.md
}

case_dotclaude_claude() {
  git init -q .
  mkdir .claude
  printf '# Nested\n\nUse tabs.\n' > .claude/CLAUDE.md
}

case_jobs_db_exists() {
  git init -q .
  ( "$job_bin" init --as someone ) >/dev/null 2>&1
}

case_second_run() {
  git init -q .
  printf '# My project\n\nAlways use tabs.\n' > CLAUDE.md
  setup_all >/dev/null
}

all_cases="empty_nogit empty_git claude_only claude_symlink_out claude_symlink_agents both_differ gitignore_no_newline gotchas_exists dotclaude_claude jobs_db_exists second_run"
if [ $# -eq 0 ]; then
  set -- $all_cases
fi

for c in "$@"; do
  echo
  echo "=================== $c"
  d=$(case_dir "$c")
  (cd "$d" && "case_$c" "$c")
  echo "### before"
  show "$d"
  echo "### setup"
  (cd "$d" && setup_all)
  echo "### after"
  show "$d"
done

echo
echo "=================== registry"
cat "$AGENTS_REGISTRY" 2>/dev/null || ls -la "$AGENTS_REGISTRY" 2>&1
