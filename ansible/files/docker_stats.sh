#!/bin/sh
# docker_stats.sh VARIANT PHASE CONTAINER OUTFILE
# Captures one docker stats snapshot and appends a CSV row to OUTFILE.
# Kept as a separate file so Ansible/Jinja never processes the Go {{ }} template strings.
VARIANT="$1"
PHASE="$2"
CONTAINER="$3"
OUTFILE="$4"

docker stats --no-stream \
  --format "${VARIANT},${PHASE},{{.CPUPerc}},{{.MemUsage}},{{.NetIO}}" \
  "$CONTAINER" \
| awk -F'[,/]' '{
    cpu=$3
    mused=$4; sub(/^ +/,"",mused)
    mlimit=$5; sub(/^ +/,"",mlimit)
    nrx=$6;   sub(/^ +/,"",nrx)
    ntx=$7;   sub(/^ +/,"",ntx)
    printf "%s,%s,%s,%s,%s,%s,%s\n",$1,$2,cpu,mused,mlimit,nrx,ntx
  }' >> "$OUTFILE"
