#!/usr/bin/env bash
# Step shell for CI: runs the step script ($1) inside the dev Vorpal
# environment. The activate script sets PATH, then runs its arguments.
set -euo pipefail

exec "$(vorpal build --path {{ .ShellQuotedDevEnvironment }})/bin/activate" bash -e "$1"
