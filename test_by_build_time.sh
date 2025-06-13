#!/bin/bash

set -e

cmd1="go build ."
cmd2="go build ."

time_command() {
  local start end
  start=$(date +%s)
  eval "$1"
  end=$(date +%s)
  awk "BEGIN {print $end - $start}"
}

t1=$(time_command "$cmd1")
t2=$(time_command "$cmd2")

echo "Time for cmd1: $t1 seconds"
echo "Time for cmd2: $t2 seconds"

threshold=0.1

diff=$(awk -v a="$t1" -v b="$t2" 'BEGIN {d = a - b; if (d < 0) d = -d; print d}')
echo "Time difference: $diff"

if (( $(awk "BEGIN {print ($diff < $threshold)}") )); then
  echo "❌ Commands took too similar time — FAIL"
  exit 1
else
  echo "✅ Commands had sufficiently different execution times"
fi
