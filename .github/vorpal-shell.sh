#!/usr/bin/env bash
# Step shell for CI: runs the step script ($1) inside the build-configs-dev
# Vorpal environment. The activate script sets PATH and the Go variables, then
# runs its arguments.
set -euo pipefail

activate="$(vorpal build --path build-configs-dev)/bin/activate"

exec "$activate" bash -e "$1"
