#!/bin/sh
set -e
# Auto-register on first start if no account is configured.
if ! cfwarp-cli status --json 2>/dev/null | grep -q '"account_configured": true'; then
    echo "[cfwarp-cli] No account found — registering…"
    cfwarp-cli register
fi
echo "[cfwarp-cli] Starting proxy…"
exec cfwarp-cli up --foreground
