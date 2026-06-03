#!/usr/bin/env bash
# Regenerate every GIF in docs/ from its .tape script in parallel.
# Requires: vhs in PATH, Go toolchain to build ./pingtop.

set -euo pipefail

# Resolve repo root regardless of where the script is invoked from.
cd "$(dirname "$0")/../.."

# Build the binary the tape scripts invoke as ./pingtop.
go build -o pingtop .

# Fan out: one vhs process per .tape file, all in background.
declare -A pids
for tape in docs/recordings/*.tape; do
    vhs "$tape" >/dev/null 2>&1 &
    pids[$!]="$tape"
    echo "started: $tape (pid $!)"
done

# Wait for each, collecting failures.
fails=()
for pid in "${!pids[@]}"; do
    if ! wait "$pid"; then
        fails+=("${pids[$pid]}")
    fi
done

if [ "${#fails[@]}" -gt 0 ]; then
    echo
    echo "FAILED:"
    printf '  %s\n' "${fails[@]}"
    exit 1
fi

echo
echo "all GIFs regenerated"
