#!/usr/bin/env bash
set -euo pipefail

echo "Building gocacheprog binary..."
go build -o gocacheprog .

CACHE_DIR=$(mktemp -d)
trap 'rm -rf "$CACHE_DIR"' EXIT

echo "Cloning a large Go project (prometheus) for realistic build test..."
git clone --depth 1 https://github.com/prometheus/prometheus.git /tmp/prometheus

GOCACHEPROG_CMD="$PWD/gocacheprog -r-urls valkey:6379 -dir $CACHE_DIR -log-metrics 2"

GOCACHE_PARENT=$(mktemp -d)

run_build() {
    local label=$1
    local time_file=$2

    echo "=== $label ==="
    local gocache
    gocache=$(mktemp -d "$GOCACHE_PARENT/gocache.XXXXXX")

    local start end duration_s
    start=$(date +%s)
    (
        cd /tmp/prometheus
        GOCACHE="$gocache" GOCACHEPROG="$GOCACHEPROG_CMD" go build -a ./cmd/prometheus
    )
    end=$(date +%s)

    duration_s=$(( end - start ))
    echo "$duration_s" > "$time_file"
    echo "$label duration: ${duration_s}s"
}

cold_time_file=$(mktemp)
warm_time_file=$(mktemp)

# 1st run – cold cache: Valkey is empty, everything compiles from scratch
# and artifacts are uploaded to Valkey.
run_build "Cold build (prometheus)" "$cold_time_file"

# 2nd run – warm cache: local GOCACHE is fresh, but Valkey still holds
# artifacts from the 1st run, so gocacheprog serves them.
run_build "Warm build (prometheus)" "$warm_time_file"

COLD_S=$(cat "$cold_time_file")
WARM_S=$(cat "$warm_time_file")

echo ""
echo "Results:"
echo "  Cold build: ${COLD_S}s"
echo "  Warm build: ${WARM_S}s"

# Guard: if cold build reports 0 seconds, timing is broken
if [ "$COLD_S" -eq 0 ]; then
    echo "FAILURE: Cold build took 0 seconds — timing is broken"
    exit 1
fi

# Assert: warm build must be at least 2× faster than cold build
if [ "$WARM_S" -le $(( COLD_S / 2 )) ]; then
    echo "SUCCESS: Warm build is at least 2x faster than cold build"
    exit 0
else
    echo "FAILURE: Warm build is NOT at least 2x faster than cold build"
    exit 1
fi
