# Alpine Linux — cloud-init

Mirror: `https://mirror.scalewithus.com`

apk indexes and packages are served under `/alpine/` (upstream: `dl-cdn.alpinelinux.org/alpine`).

Replace the host only — keep your release path (`v3.21`, `edge`, etc.).

## Rewrite repositories

```yaml
#cloud-config
runcmd:
  - |
      set -e
      sed -i -E \
        's|https?://[^[:space:]]+/alpine|https://mirror.scalewithus.com/alpine|g' \
        /etc/apk/repositories
  - [apk, update]
```

## Explicit `repositories` (example v3.21)

Change `v3.21` for your release (`v3.20`, `edge`, …):

```yaml
#cloud-config
write_files:
  - path: /etc/apk/repositories
    permissions: "0644"
    content: |
      https://mirror.scalewithus.com/alpine/v3.21/main
      https://mirror.scalewithus.com/alpine/v3.21/community
runcmd:
  - [apk, update]
```

## Verify (cloud-init)

```bash
cloud-init status --wait
apk update
```

## Live server

### One-liner

```bash
curl -fsSL https://mirror.scalewithus.com/switch-mirror.sh | sudo bash
```

### Manual

```bash
sudo sed -i -E \
  's|https?://[^[:space:]]+/alpine|https://mirror.scalewithus.com/alpine|g' \
  /etc/apk/repositories
sudo apk update
```

### Verify

```bash
curl -fsS https://mirror.scalewithus.com/healthz
sudo apk update
cat /etc/apk/repositories
```

### Revert

Restore `/etc/apk/repositories` from `/var/backups/scalewithus-mirror-*/` (if you used the switch script), then `sudo apk update`.
