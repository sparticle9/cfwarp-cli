#!/bin/sh
set -e

STATE_DIR="${CFWARP_STATE_DIR:-/var/lib/cfwarp-cli}"

# Auto-register on first start if no account is configured.
if ! cfwarp-cli status --state-dir "${STATE_DIR}" --json 2>/dev/null | grep -q '"account_configured": true'; then
    echo "[cfwarp-cli] No account found — registering…"
    cfwarp-cli register --state-dir "${STATE_DIR}"
fi

echo "[cfwarp-cli] Starting proxy…"
exec cfwarp-cli up --foreground --state-dir "${STATE_DIR}"
