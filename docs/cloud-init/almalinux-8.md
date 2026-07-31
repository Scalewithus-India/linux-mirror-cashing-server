# AlmaLinux 8 — cloud-init

Mirror: `https://mirror.scalewithus.com`

For AlmaLinux 9 see [almalinux.md](almalinux.md); for 10 see [almalinux-10.md](almalinux-10.md).

Alma 8 ships **split** repo files (`almalinux-*.repo`). Patch those — do **not** create a combined `almalinux.repo` (duplicate IDs). On EL8 the optional rebuild tree is **PowerTools** (not `CRB`).

```yaml
#cloud-config
runcmd:
  - |
      set -e
      rm -f /etc/yum.repos.d/almalinux.repo \
            /etc/yum.repos.d/*cloud_config* \
            /etc/yum.repos.d/cloud-init* \
            /etc/yum.repos.d/*.rpmnew

      for f in /etc/yum.repos.d/almalinux-*.repo; do
        [ -f "$f" ] || continue
        sed -i \
          -e 's/^mirrorlist=/#mirrorlist=/' \
          -e 's/^metalink=/#metalink=/' \
          -e 's|^#[[:space:]]*baseurl=https://repo\.almalinux\.org/almalinux|baseurl=https://mirror.scalewithus.com/almalinux|' \
          -e 's|^baseurl=https://repo\.almalinux\.org/almalinux|baseurl=https://mirror.scalewithus.com/almalinux|' \
          "$f"
      done

      dnf -y install epel-release || true
      rm -f /etc/yum.repos.d/epel-cisco-openh264.repo
      for f in /etc/yum.repos.d/epel*.repo; do
        [ -f "$f" ] || continue
        sed -i \
          -e 's/^metalink=/#metalink=/' \
          -e 's/^mirrorlist=/#mirrorlist=/' \
          -e 's|^#[[:space:]]*baseurl=https://download\.fedoraproject\.org/pub/epel|baseurl=https://mirror.scalewithus.com/epel|' \
          -e 's|^baseurl=https://download\.fedoraproject\.org/pub/epel|baseurl=https://mirror.scalewithus.com/epel|' \
          "$f"
      done

      dnf clean all
      dnf -y makecache
```

## Live server

### One-liner

```bash
curl -fsSL https://mirror.scalewithus.com/switch-mirror.sh | sudo bash
# optional EPEL:
curl -fsSL https://mirror.scalewithus.com/switch-mirror.sh | sudo bash -s -- --epel
```

### Manual

```bash
rm -f /etc/yum.repos.d/almalinux.repo /etc/yum.repos.d/*.rpmnew

for f in /etc/yum.repos.d/almalinux-*.repo; do
  sed -i \
    -e 's/^mirrorlist=/#mirrorlist=/' \
    -e 's/^metalink=/#metalink=/' \
    -e 's|^#[[:space:]]*baseurl=https://repo\.almalinux\.org/almalinux|baseurl=https://mirror.scalewithus.com/almalinux|' \
    -e 's|^baseurl=https://repo\.almalinux\.org/almalinux|baseurl=https://mirror.scalewithus.com/almalinux|' \
    "$f"
done

dnf clean all && dnf makecache
```

### Verify

```bash
curl -fsS https://mirror.scalewithus.com/healthz
grep -R '^\[baseos\]\|^\[appstream\]\|^\[powertools\]\|^\[PowerTools\]' /etc/yum.repos.d/ || true
dnf repolist
dnf makecache
```

### Revert

Restore `almalinux-*.repo` from `/var/backups/scalewithus-mirror-*/`, then `dnf makecache`.

## Verify (cloud-init)

```bash
cloud-init status --wait
dnf repolist
dnf makecache
```
