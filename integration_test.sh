#!/usr/bin/env bash
set -euo pipefail

echo "Building gocacheprog binary..."
go build -o gocacheprog .

CACHE_DIR=$(mktemp -d)
trap 'rm -rf "$CACHE_DIR"' EXIT

GOCACHEPROG_CMD="./gocacheprog -r-urls valkey:6379 -dir $CACHE_DIR"

GOCACHE_PARENT=$(mktemp -d)

run_build() {
    local label=$1
    local time_file=$2

    echo "=== $label ==="
    local gocache
    gocache=$(mktemp -d "$GOCACHE_PARENT/gocache.XXXXXX")

    local start end duration_ms
    start=$(date +%s%N)
    GOCACHE="$gocache" GOCACHEPROG="$GOCACHEPROG_CMD" go build -a std
    end=$(date +%s%N)

    duration_ms=$(( (end - start) / 1000000 ))
    echo "$duration_ms" > "$time_file"
    echo "$label duration: ${duration_ms}ms"
}

cold_time_file=$(mktemp)
warm_time_file=$(mktemp)

# 1st run – cold cache: Valkey is empty, everything compiles from scratch
# and artifacts are uploaded to Valkey.
run_build "Cold build (stdlib)" "$cold_time_file"

# 2nd run – warm cache: local GOCACHE is fresh, but Valkey still holds
# artifacts from the 1st run, so gocacheprog serves them.
run_build "Warm build (stdlib)" "$warm_time_file"

COLD_MS=$(cat "$cold_time_file")
WARM_MS=$(cat "$warm_time_file")

echo ""
echo "Results:"
echo "  Cold build: ${COLD_MS}ms"
echo "  Warm build: ${WARM_MS}ms"

# Assert: warm build must be at least 2× faster than cold build
if [ "$WARM_MS" -le $(( COLD_MS / 2 )) ]; then
    echo "SUCCESS: Warm build is at least 2x faster than cold build"
    exit 0
else
    echo "FAILURE: Warm build is NOT at least 2x faster than cold build"
    exit 1
fi
