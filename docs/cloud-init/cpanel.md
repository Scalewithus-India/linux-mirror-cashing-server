# cPanel / WHM — FastUpdate mirror

Mirror serves the official `httpupdate.cpanel.net` tree via on-demand S3 cache.

cPanel’s `HTTPUPDATE` must be a **hostname** (files are fetched from `/cpanelsync/…`, `/RPM/…` at the site root). Do **not** put a path in `HTTPUPDATE`.

cPanel install needs **≥2–4 GB RAM**. A `Increase the server's total amount of RAM` FATAL is unrelated to the mirror — resize the VM, then reinstall.

## Option A — dedicated hostname

DNS: `httpupdate.scalewithus.com` → same IP as the mirror (port **80**; HTTPS also works on this name).

```text
# /etc/cpsources.conf
HTTPUPDATE=httpupdate.scalewithus.com
```

```bash
curl -fsSI http://httpupdate.scalewithus.com/cpanelsync/
curl -fsSI http://httpupdate.scalewithus.com/RPM/
/scripts/upcp
```

## Option B — hosts entry for official name

No `cpsources.conf` change. On the cPanel server (HTTP only):

```text
# /etc/hosts
103.249.112.156  httpupdate.cpanel.net
```

```bash
getent hosts httpupdate.cpanel.net
curl -fsSI http://httpupdate.cpanel.net/cpanelsync/
curl -fsSI http://httpupdate.cpanel.net/RPM/
/scripts/upcp
```

Do **not** rely on `https://httpupdate.cpanel.net` via hosts — TLS will not match (we cannot issue a cert for `cpanel.net`).

## Debug path

```text
https://mirror.scalewithus.com/cpanel/cpanelsync/
```

## Live server

### One-liner

```bash
# Point httpupdate.cpanel.net at the mirror IP (HTTP only)
curl -fsSL https://mirror.scalewithus.com/switch-mirror.sh | sudo bash -s -- --cpanel-hosts --no-makecache
```

Also run the distro handler if you want OS packages on this mirror too (omit `--no-makecache`):

```bash
curl -fsSL https://mirror.scalewithus.com/switch-mirror.sh | sudo bash -s -- --cpanel-hosts --epel
```

### Manual

**Option A** — set `HTTPUPDATE=httpupdate.scalewithus.com` in `/etc/cpsources.conf` (DNS must resolve).

**Option B** — hosts override:

```bash
IP=$(getent ahostsv4 mirror.scalewithus.com | awk '{print $1; exit}')
echo "$IP  httpupdate.cpanel.net" | sudo tee -a /etc/hosts
curl -fsSI http://httpupdate.cpanel.net/cpanelsync/
```

### Verify

```bash
curl -fsS https://mirror.scalewithus.com/healthz
curl -fsSI http://httpupdate.cpanel.net/cpanelsync/
# or
curl -fsSI http://httpupdate.scalewithus.com/cpanelsync/
```

### Revert

Remove the `httpupdate.cpanel.net` line from `/etc/hosts` (or restore from `/var/backups/scalewithus-mirror-*/etc/hosts`), and/or clear `HTTPUPDATE` in `cpsources.conf`.
