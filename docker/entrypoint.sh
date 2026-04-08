#!/bin/sh
set -e

STATE_DIR="${CFWARP_STATE_DIR:-/home/cfwarp/.local/state/cfwarp-cli}"
RUNTIME_FAMILY="${CFWARP_RUNTIME_FAMILY:-legacy}"
TRANSPORT="${CFWARP_TRANSPORT:-wireguard}"

register_args="register --state-dir ${STATE_DIR}"
if [ "${RUNTIME_FAMILY}" = "native" ] && [ "${TRANSPORT}" = "masque" ]; then
    register_args="${register_args} --masque"
fi

# Auto-register on first start if no account is configured.
if ! cfwarp-cli status --state-dir "${STATE_DIR}" --json 2>/dev/null | grep -q '"account_configured": true'; then
    echo "[cfwarp-cli] No account found — registering…"
    # shellcheck disable=SC2086
    cfwarp-cli ${register_args}
fi

echo "[cfwarp-cli] Starting proxy…"
exec cfwarp-cli up --foreground --state-dir "${STATE_DIR}"
