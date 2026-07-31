# ScaleWithUs Linux Mirror (on-demand S3 cache)

[![GitHub stars](https://img.shields.io/github/stars/Scalewithus-India/linux-mirror-cashing-server?style=social)](https://github.com/Scalewithus-India/linux-mirror-cashing-server)

Source: [github.com/Scalewithus-India/linux-mirror-cashing-server](https://github.com/Scalewithus-India/linux-mirror-cashing-server)

Docker service for `https://mirror.scalewithus.com`. Caddy terminates TLS (Let's Encrypt); the app caches objects in S3 on demand.

## Paths

| Prefix | Upstream | Notes |
|--------|----------|--------|
| `/ubuntu/` | archive.ubuntu.com | amd64/i386 |
| `/ubuntu-ports/` | ports.ubuntu.com | arm64, armhf, etc. |
| `/ubuntu-security/` | security.ubuntu.com | optional; archive often suffices |
| `/debian/` | deb.debian.org | main, updates, backports |
| `/debian-security/` | security.debian.org | security pocket |
| `/almalinux/` | repo.almalinux.org | set explicit baseurl |
| `/rocky/` | dl.rockylinux.org | set explicit baseurl |
| `/centos-stream/` | mirror.stream.centos.org | CentOS Stream 9/10 (not EOL CentOS Linux) |
| `/alpine/` | dl-cdn.alpinelinux.org/alpine | apk main/community (and edge) |
| `/epel/` | download.fedoraproject.org/pub/epel | for EL clients |
| `/archlinux/` | geo.mirror.pkgbuild.com | official x86_64 repos |
| `/cpanel/` | httpupdate.cpanel.net | also `httpupdate.scalewithus.com` or hosts→`httpupdate.cpanel.net` |

## Run (with Let's Encrypt)

1. DNS: `mirror.scalewithus.com` and `httpupdate.scalewithus.com` → this host (ports **80** and **443** open).
2. Set S3 credentials and `ACME_EMAIL` in `.env`.
3. Start:

```bash
cd ~/linux-mirror
cp .env.example .env   # if needed; fill credentials + ACME_EMAIL
sudo docker compose up -d --build
curl -sI https://mirror.scalewithus.com/healthz
curl -sI https://mirror.scalewithus.com/ubuntu/dists/noble/InRelease
```

Caddy obtains and renews the certificate automatically. Cert data lives in the `caddy_data` Docker volume.

HTTP on port 80 is used for ACME challenges and redirects to HTTPS.

Response header `X-Cache` values include `HIT-S3`, `MISS-STORED`, `NEGATIVE`, `BYPASS-DIR` for debugging. Metrics: `https://mirror.scalewithus.com/metrics`.

---

## Client configuration

Full per-OS cloud-init / client snippets: [https://mirror.scalewithus.com/guides](https://mirror.scalewithus.com/guides) (also [docs/cloud-init.md](docs/cloud-init.md)).

Use `https://mirror.scalewithus.com` (TLS via Let's Encrypt).

### Switch an existing server

```bash
curl -fsSL https://mirror.scalewithus.com/switch-mirror.sh | sudo bash
```

Optional: `--dry-run`, `--epel`, `--cpanel-hosts`, `--no-makecache`. Details: [https://mirror.scalewithus.com/guides/switch](https://mirror.scalewithus.com/guides/switch).

### Ubuntu (amd64) — cloud-init

```yaml
#cloud-config
apt:
  primary:
    - arches: [default]
      uri: https://mirror.scalewithus.com/ubuntu
  security:
    - arches: [default]
      uri: https://mirror.scalewithus.com/ubuntu-security
```

### Ubuntu (arm64 / ports) — cloud-init

```yaml
#cloud-config
apt:
  primary:
    - arches: [arm64, armhf, ppc64el, riscv64, s390x]
      uri: https://mirror.scalewithus.com/ubuntu-ports
    - arches: [default]
      uri: https://mirror.scalewithus.com/ubuntu
  security:
    - arches: [arm64, armhf, ppc64el, riscv64, s390x]
      uri: https://mirror.scalewithus.com/ubuntu-ports
    - arches: [default]
      uri: https://mirror.scalewithus.com/ubuntu-security
```

### Debian — `sources.list`

```text
deb https://mirror.scalewithus.com/debian bookworm main contrib non-free non-free-firmware
deb https://mirror.scalewithus.com/debian bookworm-updates main contrib non-free non-free-firmware
deb https://mirror.scalewithus.com/debian-security bookworm-security main contrib non-free non-free-firmware
```

Adjust the suite (`bookworm`, `trixie`, etc.) for your release.

### Rocky Linux — drop-in repo (example BaseOS)

```ini
[baseos]
name=Rocky Linux $releasever - BaseOS
baseurl=https://mirror.scalewithus.com/rocky/$releasever/BaseOS/$basearch/os/
gpgcheck=1
enabled=1
mirrorlist=
metalink=
```

EPEL:

```ini
[epel]
name=Extra Packages for Enterprise Linux $releasever
baseurl=https://mirror.scalewithus.com/epel/$releasever/Everything/$basearch/
gpgcheck=1
enabled=1
mirrorlist=
metalink=
```

### AlmaLinux

```ini
[baseos]
name=AlmaLinux $releasever - BaseOS
baseurl=https://mirror.scalewithus.com/almalinux/$releasever/BaseOS/$basearch/os/
gpgcheck=1
enabled=1
mirrorlist=
metalink=
```

### CentOS Stream

```ini
[baseos]
name=CentOS Stream $releasever - BaseOS
baseurl=https://mirror.scalewithus.com/centos-stream/$releasever/BaseOS/$basearch/os/
gpgcheck=1
enabled=1
metalink=
```

`$releasever` is typically `9-stream` or `10-stream`. Use `/epel/` for EPEL.

### Arch Linux — `mirrorlist`

```text
Server = https://mirror.scalewithus.com/archlinux/$repo/os/$arch
```

### Alpine Linux — `/etc/apk/repositories`

Rewrite CDN hosts (keep the version path), e.g.:

```text
https://mirror.scalewithus.com/alpine/v3.21/main
https://mirror.scalewithus.com/alpine/v3.21/community
```

---

## Tunables (environment)

| Variable | Default | Purpose |
|----------|---------|---------|
| `ACME_EMAIL` | `admin@scalewithus.com` | Let's Encrypt account email |
| `S3_QUOTA_BYTES` | *(unset)* | Bucket capacity; enables S3 free space on `/metrics` |
| `S3_USAGE_REFRESH_SECONDS` | `300` | How often to re-list the bucket for usage stats |
| `METRICS_STATE_PATH` | `/var/lib/linux-mirror/metrics.json` | Persisted counter file (Docker volume `mirror_data`) |
| `METRICS_FLUSH_SECONDS` | `10` | How often dirty counters are flushed to disk |
| `METADATA_CACHE_SECONDS` | `21600` (6h) | Metadata TTL / revalidation interval |
| `PACKAGE_CACHE_SECONDS` | `15552000` (6 mo) | `Cache-Control` max-age for packages |
| `NEGATIVE_CACHE_SECONDS` | `60` | Cache upstream 404s |
| `MAX_SPOOL_BYTES` | `4GiB` | Reject oversized upstream objects |
| `MIN_TMP_FREE_BYTES` | `512MiB` | Fail miss if `/tmp` is too full |
| `UPSTREAM_TIMEOUT` | `120` | Upstream HTTP timeout (seconds) |
