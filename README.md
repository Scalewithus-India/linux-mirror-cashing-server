# ScaleWithUs Linux Mirror

[![GitHub stars](https://img.shields.io/github/stars/Scalewithus-India/linux-mirror-cashing-server?style=social)](https://github.com/Scalewithus-India/linux-mirror-cashing-server)

Source: [github.com/Scalewithus-India/linux-mirror-cashing-server](https://github.com/Scalewithus-India/linux-mirror-cashing-server)

Production instance: **https://mirror.scalewithus.com**

## What this repo does

This is an **on-demand caching mirror** for Linux (and cPanel) package trees. It is **not** a full rsync mirror that pre-downloads entire distros.

When a client requests a file (for example `/ubuntu/pool/.../foo.deb`):

1. **Local disk** (`./data/cache`) — serve immediately if present (`X-Cache: HIT-DISK`).
2. **S3** — if the object is already in the bucket, stream it (`HIT-S3`) and warm the disk cache.
3. **Upstream** — on a miss, fetch from the official source, spool to `/tmp`, upload to S3, then serve (`MISS-STORED`). Later requests hit disk or S3.

So you only store what your clients actually download. Hot packages stay on fast local NVMe/SSD; the durable shared cache is S3 (any S3-compatible API). Caddy sits in front for TLS (Let’s Encrypt) and hostname routing.

```text
Clients (apt/dnf/apk/pacman/cPanel)
        │
        ▼
   Caddy :80/:443  (TLS, Host rewrites for cPanel)
        │
        ▼
   Go mirror :8080
        │
   ┌────┴────┐
   ▼         ▼
 Disk     S3 bucket
 cache    (durable)
   │         │
   └────┬────┘
        ▼ (miss)
   Official upstreams
   (Ubuntu, Debian, Alma, …)
```

### Why run one

- Pull large packages from a nearby host / your own S3 instead of remote CDNs every time.
- One mirror URL for many distros (Ubuntu, Debian, Alma, Rocky, EPEL, Alpine, Arch, CentOS Stream, cPanel FastUpdate).
- Safe defaults: path normalization, no upstream redirect following, directory listings disabled, package size conflicts treated as immutable, spool size and concurrency caps.

### Compared to other packages / projects

Common alternatives fall into a few buckets. This project sits in the **multi-distro pull-through cache** space, with **S3 + local disk** as the storage model and a small Compose deploy.

| Project | What it is | Typical fit |
|---------|------------|-------------|
| [apt-cacher-ng](https://www.unix-ag.uni-kl.de/~bloch/acng/) | Dedicated **APT** caching proxy (Debian/Ubuntu and friends) | LAN apt-only cache; clients often set `Acquire::http::Proxy` |
| [squid-deb-proxy](https://launchpad.net/squid-deb-proxy) | Squid rules tuned for `.deb` | Avahi/LAN apt cache |
| [Squid](https://www.squid-cache.org/) / generic nginx `proxy_cache` | General HTTP cache | Flexible, but you invent package-safe TTLs, immutability, and multi-distro rules yourself |
| [apt-mirror](https://github.com/apt-mirror/apt-mirror) / [debmirror](https://manpages.debian.org/debmirror) | Proactive **full/partial** Debian/Ubuntu mirrors | Offline or “sync everything on a cron”; huge disk + bandwidth |
| `reposync` / `dnf mirror` / rsync trees | Proactive **RPM** (or whole-tree) mirrors | Same tradeoff for Alma/Rocky/EPEL: completeness vs cost |
| [Pulp](https://pulpproject.org/) (+ Foreman/Katello) | Content platform (RPM, Deb, containers, …) with sync policies | Large orgs that want RBAC, content views, scheduled syncs |
| Nexus / Artifactory | Enterprise artifact managers | Company-wide Maven/npm/Docker **and** some Linux repos; heavy ops |
| DIY nginx + scripts | Custom reverse proxy + wget/rsync | Full control; you own every edge case |

#### How this project is better *for its niche*

| Advantage | vs apt-cacher-ng / squid-deb-proxy | vs apt-mirror / debmirror / reposync | vs Pulp / Nexus / Artifactory | vs raw Squid/nginx |
|-----------|-------------------------------------|--------------------------------------|-------------------------------|--------------------|
| **Many ecosystems, one URL** | Those are apt-centric; this ships Ubuntu, Debian, Alma, Rocky, CentOS Stream, EPEL, Alpine, Arch, **and cPanel FastUpdate** under path prefixes | Full mirrors are usually one distro (or one family) per sync job | Those can do multi-format, but not as a tiny “one Compose file” mirror | You must wire every upstream yourself |
| **On-demand only** | Similar idea for apt | You don’t pay multi‑TB syncs for packages nobody installs | Lighter than standing up Pulp/Katello for “just cache what we use” | Same idea possible, not packaged |
| **S3 as durable cache** | Usually local disk only; hard to share one cache across hosts or rebuild the VM | Full mirrors rarely use object storage as the hot path | Possible, but more moving parts | DIY |
| **Local NVMe + S3 tiers** | Single local cache tier | Local tree only | Disk/S3 plugins exist; more complex | DIY |
| **Mirror-shaped URLs** | Clients often need an **HTTP proxy** setting | Clients point at a mirror URL (same as here) | Repo URLs / content apps | Proxy vs rewrite depends on setup |
| **TLS + host routing included** | Extra work (or HTTP-only LAN) | You add a web server | Built-in or separate | You configure it |
| **Package-aware defaults** | Good for apt; weak elsewhere | Sync tools don’t “serve smart”; they copy | Rich policies; heavier | Easy to cache HTML indexes or follow bad redirects |
| **Hardened pull-through** | Mature, but not this threat model | N/A (not a reverse proxy) | Enterprise hardening, different threat model | Easy to misconfigure |
| **Ops surface** | `apt install` on Debian — great for apt-only LANs | Cron + terabytes + apache/nginx | Multi-service platform | Config sprawl |

Concrete behaviours that matter in production:

- **Disk → S3 → upstream** with clear `X-Cache` headers (`HIT-DISK` / `HIT-S3` / `MISS-STORED`) for debugging.
- **Metadata vs package TTLs** (revalidate indexes; treat packages as immutable).
- **Refuses upstream redirects**, disables directory listings, caps spool size and concurrent upstream fetches.
- **cPanel**: hostname rewrite so FastUpdate can use `HTTPUPDATE=` or a hosts override (not something apt-cacher-ng covers).
- **Client switcher** (`switch-mirror.sh`) + cloud-init docs for live fleets.

#### When to pick something else

| Choose… | When… |
|---------|--------|
| **apt-cacher-ng** | You only care about Debian/Ubuntu on a LAN and want the smallest apt-specific install |
| **apt-mirror / rsync / reposync** | You need a **complete** offline/air-gapped tree, not “what was requested” |
| **Pulp / Katello / Nexus** | You need content lifecycle, RBAC, promotion between environments, or many non-OS artifact types |
| **This repo** | You want a **public or private multi-distro HTTPS mirror**, S3-backed, on-demand, Compose-deployable, including RPM/apk/pacman/cPanel — without running a full content platform |

### Request path layout

Public URLs use a prefix per distro. That prefix maps to an official upstream base URL. S3 object keys match the public path **without** a leading slash (for example `ubuntu/dists/noble/InRelease`).

| Prefix | Upstream |
|--------|----------|
| `/ubuntu/` | archive.ubuntu.com |
| `/ubuntu-ports/` | ports.ubuntu.com |
| `/ubuntu-security/` | security.ubuntu.com |
| `/debian/` | deb.debian.org |
| `/debian-security/` | security.debian.org |
| `/almalinux/` | repo.almalinux.org |
| `/rocky/` | dl.rockylinux.org |
| `/centos-stream/` | mirror.stream.centos.org |
| `/alpine/` | dl-cdn.alpinelinux.org/alpine |
| `/epel/` | download.fedoraproject.org/pub/epel |
| `/archlinux/` | geo.mirror.pkgbuild.com |
| `/cpanel/` | httpupdate.cpanel.net |

cPanel clients expect files at the **host root** (`/cpanelsync/…`, `/RPM/…`). Caddy rewrites those onto `/cpanel{uri}` for `httpupdate.scalewithus.com` and `http://httpupdate.cpanel.net` (hosts-file override). Use **HTTP** for the cPanel hosts name — you cannot get a TLS cert for `cpanel.net`.

### Cache behaviour (short)

| Header `X-Cache` | Meaning |
|------------------|---------|
| `HIT-DISK` | Served from local disk cache |
| `HIT-S3` | Served from S3 |
| `MISS-STORED` | Fetched upstream, stored in S3, served |
| `NEGATIVE` | Recent upstream 404 (short TTL) |
| `DIR-DISABLED` | Directory listing blocked |
| `UPSTREAM-REDIRECT` | Upstream returned 3xx (refused) |

- **Metadata** (InRelease, Packages, repomd.xml, …): revalidated on a shorter TTL (default 6h).
- **Packages** (`.deb`, `.rpm`, …): treated as immutable; long `Cache-Control` (default 6 months).
- **Disk budget**: by default `free_disk − 15 GiB` under `./data/cache`; set `LOCAL_CACHE_BYTES=0` to disable.

### Repo layout

| Path | Role |
|------|------|
| `cmd/mirror/` | Go process entrypoint |
| `internal/{config,upstream,store,mirror,diskcache,metrics,web}/` | Config, routing, S3, disk cache, HTTP handlers, site |
| `web/` | HTML templates + static assets |
| `docs/` | Per-OS client / cloud-init guides (also served at `/guides`) |
| `scripts/switch-mirror.sh` | Point an existing server at this mirror |
| `docker-compose.yml` | `mirror` + `caddy` stack |
| `Caddyfile` | Hostnames, TLS, cPanel rewrites |
| `data/cache/` | Local object cache on the host (gitignored) |

---

## How to host it

### Requirements

| Requirement | Notes |
|-------------|--------|
| Linux host | Enough disk for `./data/cache` (NVMe/SSD preferred) + Docker |
| Docker + Compose | Plugin `docker compose` |
| Public IPv4 | Ports **80** and **443** reachable from the internet (ACME + clients) |
| DNS | At least one A/AAAA record for your mirror hostname |
| S3-compatible bucket | Credentials with read/write on the bucket; path-style addressing supported |

Suggested sizing: multi-core CPU, ≥4 GiB RAM, tens of GiB free disk for the local cache (the compose stack mounts `./data/cache` and keeps a 15 GiB free-space reserve by default). Spools use a tmpfs `/tmp` (8 GiB in compose) for upstream downloads before S3 upload.

### 1. Clone and configure

```bash
git clone https://github.com/Scalewithus-India/linux-mirror-cashing-server.git linux-mirror
cd linux-mirror
cp .env.example .env
```

Edit `.env` — **required**:

```bash
AWS_ACCESS_KEY_ID=...
AWS_SECRET_ACCESS_KEY=...
S3_ENDPOINT=https://s3.example.com          # your S3 API endpoint
S3_BUCKET=linux-mirrors
S3_REGION=us-east-1
S3_ADDRESSING=path                          # or virtual
ACME_EMAIL=you@example.com                  # Let's Encrypt account email
```

Optional but useful:

```bash
S3_QUOTA_BYTES=10995116277760               # bucket capacity; enables free-space on /metrics
LOCAL_CACHE_RESERVE_BYTES=16106127360       # keep 15 GiB free on the data volume
# LOCAL_CACHE_BYTES=0                       # disable disk tier
# LOCAL_CACHE_BYTES=107374182400            # or fix disk cache at 100 GiB
```

### 2. DNS

Point records at this host’s public IP:

| Name | Purpose |
|------|---------|
| `mirror.example.com` | Main HTTPS mirror (required) |
| `httpupdate.example.com` | Optional cPanel `HTTPUPDATE` hostname (must resolve) |

If you use names other than `mirror.scalewithus.com` / `httpupdate.scalewithus.com`, edit `Caddyfile` to match **before** starting Compose. Example:

```caddy
mirror.example.com {
	import mirror_proxy
}

http://httpupdate.example.com {
	rewrite * /cpanel{uri}
	import mirror_proxy
}

httpupdate.example.com {
	rewrite * /cpanel{uri}
	import mirror_proxy
}

# Optional: hosts-file clients that keep the official name (HTTP only)
http://httpupdate.cpanel.net {
	rewrite * /cpanel{uri}
	import mirror_proxy
}
```

Also update client docs / `switch-mirror.sh` defaults if you publish a public switcher for your own domain.

### 3. Firewall

Allow inbound:

- **80/tcp** — HTTP (ACME HTTP-01 + cPanel FastUpdate)
- **443/tcp** (and **443/udp** if you want HTTP/3) — HTTPS clients

Do **not** publish the Go app’s `:8080` on the public interface; Compose only `expose`s it to Caddy on the internal network.

### 4. Start the stack

```bash
sudo docker compose up -d --build
sudo docker compose ps
sudo docker compose logs -f mirror caddy
```

What starts:

| Service | Role |
|---------|------|
| `linux-mirror` | Go cache proxy on `:8080` |
| `linux-mirror-caddy` | Publishes `:80` / `:443`, obtains/renews certs into volume `caddy_data` |

Persistent volumes:

| Volume / path | Contents |
|---------------|----------|
| `mirror_data` | Persisted `/metrics` counters (`metrics.json`) |
| `caddy_data` / `caddy_config` | Certificates and Caddy state |
| `./data/cache` | Local NVMe/SSD object cache |

### 5. Verify

```bash
# App health (via HTTPS)
curl -fsS https://mirror.example.com/healthz

# Metadata pull (should be HIT-S3 or MISS-STORED first time, then HIT-DISK)
curl -sI https://mirror.example.com/ubuntu/dists/noble/InRelease | grep -iE 'HTTP|x-cache|content-length'

# Metrics JSON
curl -fsS https://mirror.example.com/metrics | head

# cPanel rewrite (if DNS + Caddyfile configured)
curl -fsSI http://httpupdate.example.com/cpanelsync/
```

Expect `X-Cache` values as in the table above. First miss of a large package can take a while (upstream download + S3 put); concurrent misses are capped by `MAX_CONCURRENT_SPOOLS` (default 3).

### 6. Point clients at the mirror

**Existing servers** (rewrites apt/dnf/apk/pacman where possible; optional EPEL / cPanel hosts):

```bash
curl -fsSL https://mirror.example.com/switch-mirror.sh | sudo bash
# flags: --dry-run --epel --cpanel-hosts --no-makecache
```

**First boot / cloud-init** and per-OS snippets: [docs/cloud-init.md](docs/cloud-init.md) (also https://mirror.scalewithus.com/guides on the production host).

Examples:

```yaml
# Ubuntu cloud-init
#cloud-config
apt:
  primary:
    - arches: [default]
      uri: https://mirror.example.com/ubuntu
  security:
    - arches: [default]
      uri: https://mirror.example.com/ubuntu-security
```

```text
# Debian sources.list
deb https://mirror.example.com/debian bookworm main contrib non-free non-free-firmware
deb https://mirror.example.com/debian bookworm-updates main contrib non-free non-free-firmware
deb https://mirror.example.com/debian-security bookworm-security main contrib non-free non-free-firmware
```

```ini
# Alma / Rocky — set explicit baseurl; clear mirrorlist/metalink
baseurl=https://mirror.example.com/almalinux/$releasever/BaseOS/$basearch/os/
```

```text
# cPanel — Option A (dedicated hostname in /etc/cpsources.conf)
HTTPUPDATE=httpupdate.example.com

# Option B — /etc/hosts on the cPanel server (HTTP only)
YOUR.MIRROR.IP  httpupdate.cpanel.net
```

### 7. Day-2 operations

```bash
# Redeploy after code or Caddyfile changes
cd ~/linux-mirror && sudo docker compose up -d --build

# Logs
sudo docker compose logs -f --tail=200 mirror
sudo docker compose logs -f --tail=200 caddy

# Disk cache size on the host
sudo du -sh data/cache

# Clear one cached object (forces next request to S3 or upstream)
sudo rm -f data/cache/ubuntu/dists/noble/InRelease \
           data/cache/ubuntu/dists/noble/InRelease.ctype
```

Caddy renews certificates automatically. Metrics survive container restarts via the `mirror_data` volume. S3 objects remain until you delete them in the bucket (the app does not garbage-collect S3 by default; disk cache LRU-evicts when over budget).

### 8. Security notes for operators

- Keep S3 keys only in `.env` (never commit). Restrict the IAM/key to that bucket.
- The mirror refuses upstream **redirects** and does not proxy HTML directory indexes.
- Oversized upstream objects are rejected (`MAX_SPOOL_BYTES`, default 2 GiB).
- Prefer running behind Caddy as shipped; do not expose `:8080` publicly without an equivalent reverse proxy and rate limits if needed.

---

## Environment reference

| Variable | Default | Purpose |
|----------|---------|---------|
| `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` | *(required)* | S3 credentials |
| `S3_ENDPOINT` | `https://s3.scalewithus.com` | S3 API base URL |
| `S3_BUCKET` | `linux-mirrors` | Bucket name |
| `S3_REGION` | `us-east-1` | Region string for signing |
| `S3_ADDRESSING` | `path` | `path` or `virtual` |
| `S3_QUOTA_BYTES` | *(unset)* | Bucket capacity for `/metrics` free space |
| `S3_USAGE_REFRESH_SECONDS` | `300` | How often to re-list bucket usage |
| `ACME_EMAIL` | `admin@scalewithus.com` | Let’s Encrypt email (Caddy) |
| `METADATA_CACHE_SECONDS` | `21600` (6h) | Metadata revalidation interval |
| `PACKAGE_CACHE_SECONDS` | `15552000` (6 mo) | `Cache-Control` max-age for packages |
| `NEGATIVE_CACHE_SECONDS` | `60` | Cache upstream 404s |
| `MAX_SPOOL_BYTES` | `2GiB` | Max upstream object size |
| `MAX_CONCURRENT_SPOOLS` | `3` | Parallel upstream→S3 fetches |
| `MIN_TMP_FREE_BYTES` | `512MiB` | Refuse miss if `/tmp` too full |
| `UPSTREAM_TIMEOUT` | `120` | Upstream HTTP timeout (seconds) |
| `HEAD_CACHE_MAX` | `50000` | In-memory S3 head cache entries |
| `LOCAL_CACHE_DIR` | `/app/data/cache` | Disk cache root in container |
| `LOCAL_CACHE_RESERVE_BYTES` | `15GiB` | Free space kept when auto-sizing |
| `LOCAL_CACHE_BYTES` | *(auto)* | Fixed cap; empty = free−reserve; `0` = off |
| `METRICS_STATE_PATH` | `/var/lib/linux-mirror/metrics.json` | Persisted counters |
| `METRICS_FLUSH_SECONDS` | `10` | Counter flush interval |
| `LISTEN` | `:8080` | App listen address |
| `LOG_LEVEL` | `INFO` | `DEBUG` / `INFO` / `WARN` / `ERROR` |

---

## Development (without full TLS)

You can run the Go binary locally against real S3 credentials, but production TLS and host routing are intended via Compose + Caddy:

```bash
export $(grep -v '^#' .env | xargs)
go run ./cmd/mirror
# listen on :8080 — put a reverse proxy in front for HTTPS
```

Build image only:

```bash
sudo docker compose build mirror
```
