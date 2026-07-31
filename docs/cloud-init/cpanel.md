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
