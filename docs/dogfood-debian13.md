# Debian 13 dogfood runbook

This runbook prepares a **remote Debian 13 host with Docker** for side-by-side dogfooding:

- **stable path:** WireGuard/WARP SOCKS5 proxy
- **experimental path:** native MASQUE SOCKS5 proxy
- **consumer path:** local sing-box router forwarding selected traffic into one of those localhost proxies

## Readiness call

Short version:

- **WireGuard/WARP:** ready for real dogfood use
- **native MASQUE:** ready for opt-in verification, comparison, and light self-hosted experiments, but still **experimental**

That means the recommended operator posture is:

1. depend on **WireGuard/WARP** for the daemon you actually care about
2. run **MASQUE** alongside it on another localhost port
3. compare behavior, logs, and `cdn-cgi/trace`
4. only switch real traffic to MASQUE after your own verification window

See also:

- `docs/status/2026-04-native-masque-status.md`
- `docs/native-masque-vs-singbox-review.md`

## What this repo now provides

### Release/publish

- `.github/workflows/docker.yml`
  - publishes multi-arch GHCR images for:
    - `ghcr.io/sparticle9/cfwarp-cli:latest`
    - `ghcr.io/sparticle9/cfwarp-cli:latest-debian`
  - also publishes branch/tag/sha variants

### Deployment assets

- `deploy/docker-compose.dogfood.yml`
  - dual-proxy stack
  - uses the same published cfwarp image for both protocol lanes
  - binds SOCKS listeners only on `127.0.0.1`
  - uses per-service bind-mounted state directories so `settings.json` controls container behavior by default
  - optional `canary` profile runs curl-based trace containers through each proxy
- `deploy/dogfood.env.example`
  - compose env file template
- `deploy/daemon-proxy.env.example`
  - example host-daemon proxy env file

### CLI/Ansible operations

- `ansible/dogfood-deploy.yml`
  - copies compose files to remote host
  - writes per-service `settings.json` files into bind-mounted state directories
  - starts both proxies on localhost-only nonstandard ports `16080` and `16081`
  - can inject two local SOCKS outbounds into sing-box config when fragment files are available
  - adds `geosite-google-deepmind` rule-set routing to the managed cfwarp outbound
  - removes the old periodic observer timer/service if it exists
- `ansible/dogfood-status.yml`
  - on-demand inspection only
  - shows compose state, `docker inspect`, `docker stats`, `cfwarp-cli status --json`, backend runtime files, backend stderr logs, sing-box service state, and sing-box journal output

## Option A — deploy with Ansible

Copy `ansible/inventory.ini.example` to `ansible/inventory.ini`, add your remote host under `[warp]`, then run:

```bash
ansible-playbook -i ansible/inventory.ini ansible/dogfood-deploy.yml --limit <host>
```

If you want to try the Debian image variant explicitly:

```bash
ansible-playbook -i ansible/inventory.ini ansible/dogfood-deploy.yml --limit <host> \
  -e cfwarp_image=ghcr.io/sparticle9/cfwarp-cli:latest-debian \
  -e cfwarp_wireguard_state_dir=/home/nonroot/.local/state/cfwarp-cli \
  -e cfwarp_masque_state_dir=/home/nonroot/.local/state/cfwarp-cli
```

Then inspect the running stack:

```bash
ansible-playbook -i ansible/inventory.ini ansible/dogfood-status.yml --limit <host>
```

Or include live trace verification:

```bash
ansible-playbook -i ansible/inventory.ini ansible/dogfood-status.yml --limit <host> \
  -e dogfood_verify_trace=true
```

## Option B — deploy manually on the remote host

Copy the files under `deploy/` to the host, then:

```bash
cd /opt/cfwarp-dogfood
cp dogfood.env.example .env
cp docker-compose.dogfood.yml docker-compose.yml
mkdir -p state/wireguard state/masque

# Write settings.json into each state directory before first start.
# Those files control transport/access/caps/rotation behavior by default.

docker compose --env-file .env -f docker-compose.yml up -d
```

To also run the canary containers:

```bash
docker compose --profile canary --env-file .env -f docker-compose.yml up -d
```

## Verify from the remote host CLI

### 1. Check container state

```bash
docker compose -f /opt/cfwarp-dogfood/docker-compose.yml ps
```

### 2. Check `cfwarp-cli` runtime status inside each container

Alpine image default paths:

```bash
docker exec cfwarp-warp cfwarp-cli status --json \
  --state-dir /home/cfwarp/.local/state/cfwarp-cli

docker exec cfwarp-masque cfwarp-cli status --json \
  --state-dir /home/cfwarp/.local/state/cfwarp-cli

docker exec cfwarp-warp cfwarp-cli daemon ctl status \
  --state-dir /home/cfwarp/.local/state/cfwarp-cli
```

Debian image default paths:

```bash
docker exec cfwarp-warp cfwarp-cli status --json \
  --state-dir /home/nonroot/.local/state/cfwarp-cli

docker exec cfwarp-masque cfwarp-cli status --json \
  --state-dir /home/nonroot/.local/state/cfwarp-cli
```

### 3. Check backend runtime files directly when status output is misleading

```bash
docker exec cfwarp-masque sh -lc 'cat /home/cfwarp/.local/state/cfwarp-cli/run/runtime.json'
docker exec cfwarp-masque sh -lc 'tail -n 40 /home/cfwarp/.local/state/cfwarp-cli/logs/backend.stderr.log'

docker exec cfwarp-warp sh -lc 'tail -n 40 /home/cfwarp/.local/state/cfwarp-cli/logs/backend.stderr.log'
```

### 4. Check actual egress through each localhost SOCKS listener

```bash
curl -fsSL --proxy socks5h://127.0.0.1:16080 https://www.cloudflare.com/cdn-cgi/trace
curl -fsSL --proxy socks5h://127.0.0.1:16081 https://www.cloudflare.com/cdn-cgi/trace
```

Expected output includes:

```text
warp=on
```

## Route selected traffic through local sing-box

When sing-box is using fragment files under `/etc/sing-box`, the deploy playbook can manage them with `become` and inject:

- outbound `cfwarp-warp-local` => `socks5://127.0.0.1:16080`
- outbound `cfwarp-masque-local` => `socks5://127.0.0.1:16081`
- rule-set `geosite-google-deepmind`
- route rule sending that rule-set to the managed target outbound

Default managed target outbound:

- `cfwarp-masque-local`

You can override that at deploy time:

```bash
ansible-playbook -i ansible/inventory.ini ansible/dogfood-deploy.yml --limit <host> \
  -e singbox_rule_target_outbound=cfwarp-warp-local
```

### Host daemon example

If you want a daemon to bypass sing-box and point directly at one cfwarp proxy, use `deploy/daemon-proxy.env.example` as the basis for an EnvironmentFile.

Example with systemd override:

```bash
sudo install -d -m 0755 /etc/example-daemon
sudo cp deploy/daemon-proxy.env.example /etc/example-daemon/proxy.env
sudo systemctl edit example-daemon.service
```

Drop-in content:

```ini
[Service]
EnvironmentFile=/etc/example-daemon/proxy.env
```

Then restart the daemon:

```bash
sudo systemctl daemon-reload
sudo systemctl restart example-daemon.service
```

Change the proxy port in the env file:

- `16080` => stable WireGuard/WARP path
- `16081` => experimental MASQUE path

### Container daemon example

The compose file already contains optional `trace-wireguard` and `trace-masque` services under the `canary` profile. Those are simple stand-ins for another local daemon that routes through the proxy using `ALL_PROXY`.

## On-demand monitoring loop

There is no longer a background observer service.

Use these commands when you want status:

```bash
ansible-playbook -i ansible/inventory.ini ansible/dogfood-status.yml --limit <host> \
  -e dogfood_verify_trace=true

docker inspect --format '{{json .State}}' cfwarp-warp
docker inspect --format '{{json .State}}' cfwarp-masque

docker stats --no-stream cfwarp-warp cfwarp-masque

docker logs --tail 100 cfwarp-warp
docker logs --tail 100 cfwarp-masque

systemctl status sing-box --no-pager
journalctl -u sing-box -n 100 --no-pager

docker exec cfwarp-masque sh -lc 'cat /home/cfwarp/.local/state/cfwarp-cli/run/runtime.json'
docker exec cfwarp-masque sh -lc 'tail -n 40 /home/cfwarp/.local/state/cfwarp-cli/logs/backend.stderr.log'

docker exec cfwarp-warp cfwarp-cli status --json --require-account --require-running --require-reachable \
  --state-dir /home/cfwarp/.local/state/cfwarp-cli

docker exec cfwarp-warp cfwarp-cli daemon ctl rotate \
  --state-dir /home/cfwarp/.local/state/cfwarp-cli
```

For the Debian image, swap the state dir path to `/home/nonroot/.local/state/cfwarp-cli`.

## Current caution on MASQUE

Native MASQUE is good enough to dogfood **next to** WireGuard, but not yet something I would recommend as the only path for a dependency-critical daemon.

Reason:

- startup retry behavior is still an open issue
- workload behavior still varies more than the stable WireGuard path
- the architecture is promising, but not yet fully settled

So the practical recommendation is:

- **ship WireGuard as the safe lane**
- **run MASQUE as the comparison lane**
- **promote MASQUE only after your own repeated verification**
