#!/usr/bin/env bash
#
# ac-commands: run the repository's checks and print each result for the
# verifier.
#
# Wired as the ac-commands pre-gate on verify-ac. It runs at claim, and its
# output lands in the step's context bundle. verify-ac cannot settle a
# criterion such as "the tests pass" by reading a diff, so without recorded
# command results it returns unverifiable.
#
# It reports and does not judge: it prints what ran and the exit status of
# each command, then exits with the worst status among them. A non-zero exit
# is recorded as data and does not refuse the claim.
#
# The repository root comes from the invoking directory first, because the
# engine runs gates with the step's worktree as cwd. The script-relative path
# is only the fallback for a manual run outside any git repository.

set -euo pipefail

cd "$(git rev-parse --show-toplevel 2>/dev/null || echo "$(dirname "${BASH_SOURCE[0]}")/../..")"

worst=0

run() {
  local label="$1"
  shift
  echo "--- $label: $* ---"
  local rc=0
  "$@" 2>&1 | tail -20 || rc=${PIPESTATUS[0]}
  echo "[$label] exit $rc"
  if [ "$rc" -gt "$worst" ]; then
    worst="$rc"
  fi
  echo
}

echo "=== ac-commands: recorded results for the verifier ==="
echo "commit: $(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
echo "tree:   $(git status --porcelain 2>/dev/null | wc -l | tr -d ' ') uncommitted path(s)"
echo "just:   $(command -v just || echo unknown)"
echo "go:     $(command -v go || echo unknown)"
echo

run build just build
run test just test
run lint just lint

echo "worst exit: $worst"
exit "$worst"
