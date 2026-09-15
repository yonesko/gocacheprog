#!/usr/bin/env bash
set -euo pipefail

# Build the gocacheprog binary that will act as the cache proxy
echo "Building gocacheprog binary..."
go build -o gocacheprog .

# Temporary directory used by gocacheprog for local file-system cache
CACHE_DIR=$(mktemp -d)
trap 'rm -rf "$CACHE_DIR"' EXIT

GOCACHEPROG_CMD="./gocacheprog -r-urls valkey:6379 -dir $CACHE_DIR"

# Helper: run a timed go build -a and save elapsed ms to a file
run_build() {
    local label=$1
    local time_file=$2

    echo "=== $label ==="
    go clean -cache
    rm -rf "$CACHE_DIR"/*

    local start end duration_ms
    start=$(date +%s%N)
    GOCACHEPROG="$GOCACHEPROG_CMD" go build -a .
    end=$(date +%s%N)

    duration_ms=$(( (end - start) / 1000000 ))
    echo "$duration_ms" > "$time_file"
    echo "$label duration: ${duration_ms}ms"
}

cold_time_file=$(mktemp)
warm_time_file=$(mktemp)

# 1st run – cold cache: nothing is in Valkey yet, so every artifact is
# compiled from scratch and then uploaded.
run_build "Cold build" "$cold_time_file"

# 2nd run – warm cache: Go's local cache is cleared, but Valkey still holds
# the artifacts from the 1st run, so gocacheprog can serve them.
run_build "Warm build" "$warm_time_file"

COLD_MS=$(cat "$cold_time_file")
WARM_MS=$(cat "$warm_time_file")

echo ""
echo "Results:"
echo "  Cold build: ${COLD_MS}ms"
echo "  Warm build: ${WARM_MS}ms"

# Assert: warm build must be at least 2x faster than cold build
if [ "$WARM_MS" -le $(( COLD_MS / 2 )) ]; then
    echo "SUCCESS: Warm build is at least 2x faster than cold build"
    exit 0
else
    echo "FAILURE: Warm build is NOT at least 2x faster than cold build"
    exit 1
fi
