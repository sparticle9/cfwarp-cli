# Debian 13 dogfood runbook

This runbook prepares a **remote Debian 13 host with Docker** for side-by-side dogfooding:

- **stable path:** WireGuard/WARP SOCKS5 proxy
- **experimental path:** native MASQUE SOCKS5 proxy
- **consumer path:** another local daemon or container routing through one of those proxies

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
  - binds SOCKS listeners only on `127.0.0.1`
  - optional `canary` profile runs curl-based trace containers through each proxy
- `deploy/dogfood.env.example`
  - compose env file template
- `deploy/daemon-proxy.env.example`
  - example host-daemon proxy env file

### CLI/Ansible operations

- `ansible/dogfood-deploy.yml`
  - copies compose files to remote host
  - starts both proxies
  - verifies `warp=on`
- `ansible/dogfood-status.yml`
  - shows compose state, `docker inspect`, `cfwarp-cli status --json`, optional live trace, and recent logs

## Option A — deploy with Ansible

Copy `ansible/inventory.ini.example` to `ansible/inventory.ini`, add your remote host under `[warp]`, then run:

```bash
ansible-playbook -i ansible/inventory.ini ansible/dogfood-deploy.yml --limit warp
```

If you want to try the Debian image variant explicitly:

```bash
ansible-playbook -i ansible/inventory.ini ansible/dogfood-deploy.yml --limit warp \
  -e cfwarp_wireguard_image=ghcr.io/sparticle9/cfwarp-cli:latest-debian \
  -e cfwarp_masque_image=ghcr.io/sparticle9/cfwarp-cli:latest-debian \
  -e cfwarp_wireguard_state_dir=/home/nonroot/.local/state/cfwarp-cli \
  -e cfwarp_masque_state_dir=/home/nonroot/.local/state/cfwarp-cli
```

Then inspect the running stack:

```bash
ansible-playbook -i ansible/inventory.ini ansible/dogfood-status.yml --limit warp
```

Or include live trace verification:

```bash
ansible-playbook -i ansible/inventory.ini ansible/dogfood-status.yml --limit warp \
  -e dogfood_verify_trace=true
```

## Option B — deploy manually on the remote host

Copy the files under `deploy/` to the host, then:

```bash
cd /opt/cfwarp-dogfood
cp dogfood.env.example .env
cp docker-compose.dogfood.yml docker-compose.yml

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
```

Debian image default paths:

```bash
docker exec cfwarp-warp cfwarp-cli status --json \
  --state-dir /home/nonroot/.local/state/cfwarp-cli

docker exec cfwarp-masque cfwarp-cli status --json \
  --state-dir /home/nonroot/.local/state/cfwarp-cli
```

### 3. Check actual egress through each localhost SOCKS listener

```bash
curl -fsSL --proxy socks5h://127.0.0.1:18080 https://www.cloudflare.com/cdn-cgi/trace
curl -fsSL --proxy socks5h://127.0.0.1:18081 https://www.cloudflare.com/cdn-cgi/trace
```

Expected output includes:

```text
warp=on
```

## Route another local daemon through the proxy

### Host daemon example

Use `deploy/daemon-proxy.env.example` as the basis for an EnvironmentFile.

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

- `18080` => stable WireGuard/WARP path
- `18081` => experimental MASQUE path

### Container daemon example

The compose file already contains optional `trace-wireguard` and `trace-masque` services under the `canary` profile. Those are simple stand-ins for another local daemon that routes through the proxy using `ALL_PROXY`.

## Monitoring loop

Useful commands during dogfooding:

```bash
docker inspect --format '{{json .State}}' cfwarp-warp
docker inspect --format '{{json .State}}' cfwarp-masque

docker logs --tail 100 cfwarp-warp
docker logs --tail 100 cfwarp-masque

docker exec cfwarp-warp cfwarp-cli status --json --require-account --require-running --require-reachable \
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
