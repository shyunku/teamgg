#!/usr/bin/env bash
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

source "$SCRIPT_DIR/stop.sh"
source "$SCRIPT_DIR/build.sh"
source "$SCRIPT_DIR/daemon.sh"