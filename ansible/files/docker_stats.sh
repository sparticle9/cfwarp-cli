#!/bin/sh
# docker_stats.sh VARIANT PHASE CONTAINER OUTFILE
# Captures one docker stats snapshot and appends a CSV row to OUTFILE.
# Kept as a separate file so Ansible/Jinja never processes the Docker {{ }} templates.
VARIANT="$1"
PHASE="$2"
CONTAINER="$3"
OUTFILE="$4"
TS="$(date +%s)"

docker stats --no-stream \
  --format "{{.CPUPerc}},{{.MemUsage}},{{.NetIO}}" \
  "$CONTAINER" \
| awk -F'[,/]' -v ts="$TS" -v variant="$VARIANT" -v phase="$PHASE" '{
    cpu=$1;   sub(/^ +/,"",cpu)
    mused=$2; sub(/^ +/,"",mused)
    mlimit=$3; sub(/^ +/,"",mlimit)
    nrx=$4;   sub(/^ +/,"",nrx)
    ntx=$5;   sub(/^ +/,"",ntx)
    printf "%s,%s,%s,%s,%s,%s,%s,%s\n", ts, variant, phase, cpu, mused, mlimit, nrx, ntx
  }' >> "$OUTFILE"
