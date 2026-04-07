#!/bin/sh
# docker_stats.sh VARIANT PHASE CONTAINER OUTFILE
# Accurate per-container sampling using host cgroup counters for CPU/memory.
# Net I/O still comes from docker stats. Output rows are for the target container only.
set -eu

VARIANT="$1"
PHASE="$2"
CONTAINER="$3"
OUTFILE="$4"
TS_US="$(python3 - <<'PY'
import time
print(int(time.time() * 1_000_000))
PY
)"
STATEFILE="/tmp/docker-stats-${CONTAINER}-${PHASE}.state"

PID="$(docker inspect -f '{{.State.Pid}}' "$CONTAINER")"
CGROUP_REL="$(awk -F: '$1==0{print $3}' /proc/$PID/cgroup)"
CGROUP="/sys/fs/cgroup${CGROUP_REL}"
CPU_USAGE_US="$(awk '/usage_usec/ {print $2}' "$CGROUP/cpu.stat")"
MEM_USED_BYTES="$(cat "$CGROUP/memory.current")"
MEM_LIMIT_RAW="$(cat "$CGROUP/memory.max")"
if [ "$MEM_LIMIT_RAW" = "max" ]; then
  MEM_LIMIT_BYTES=0
else
  MEM_LIMIT_BYTES="$MEM_LIMIT_RAW"
fi

CPU_PCT="0.00"
if [ -f "$STATEFILE" ]; then
  PREV_TS_US="$(cut -d, -f1 "$STATEFILE")"
  PREV_CPU_USAGE_US="$(cut -d, -f2 "$STATEFILE")"
  DELTA_TS=$((TS_US - PREV_TS_US))
  DELTA_CPU=$((CPU_USAGE_US - PREV_CPU_USAGE_US))
  if [ "$DELTA_TS" -gt 0 ] && [ "$DELTA_CPU" -ge 0 ]; then
    CPU_PCT="$(python3 - <<PY
from decimal import Decimal, ROUND_HALF_UP
usage = Decimal(${DELTA_CPU})
wall = Decimal(${DELTA_TS})
pct = (usage / wall) * Decimal(100)
print(pct.quantize(Decimal('0.01'), rounding=ROUND_HALF_UP))
PY
)"
  fi
fi
printf '%s,%s\n' "$TS_US" "$CPU_USAGE_US" > "$STATEFILE"

NET_IO="$(docker stats --no-stream --format '{{.NetIO}}' "$CONTAINER" || echo '0B / 0B')"
NET_RX="$(printf '%s' "$NET_IO" | awk -F'/' '{gsub(/^ +| +$/,"",$1); print $1}')"
NET_TX="$(printf '%s' "$NET_IO" | awk -F'/' '{gsub(/^ +| +$/,"",$2); print $2}')"

printf '%s,%s,%s,%s%%,%s,%s,%s,%s\n' \
  "$TS_US" "$VARIANT" "$PHASE" "$CPU_PCT" "$MEM_USED_BYTES" "$MEM_LIMIT_BYTES" "$NET_RX" "$NET_TX" >> "$OUTFILE"
