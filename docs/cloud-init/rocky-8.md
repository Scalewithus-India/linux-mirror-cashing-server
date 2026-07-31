# Rocky Linux 8 — cloud-init

For Rocky 9 see [rocky.md](rocky.md); for 10 see [rocky-10.md](rocky-10.md).

Mirror: `https://mirror.scalewithus.com`

Patches stock `rocky*.repo` files to use this mirror (disables mirrorlist/metalink).

```yaml
#cloud-config
runcmd:
  - |
      set -e
      rm -f /etc/yum.repos.d/*cloud_config* /etc/yum.repos.d/cloud-init*

      for f in /etc/yum.repos.d/rocky*.repo; do
        [ -f "$f" ] || continue
        sed -i \
          -e 's/^mirrorlist=/#mirrorlist=/' \
          -e 's/^metalink=/#metalink=/' \
          -e 's|^#[[:space:]]*baseurl=https\?://dl\.rockylinux\.org/\$contentdir|baseurl=https://mirror.scalewithus.com/rocky|' \
          -e 's|^baseurl=https\?://dl\.rockylinux\.org/\$contentdir|baseurl=https://mirror.scalewithus.com/rocky|' \
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

## Verify (cloud-init)

```bash
cloud-init status --wait
grep -R '^\[baseos\]\|^\[appstream\]\|^\[crb\]' /etc/yum.repos.d/
dnf repolist
dnf makecache
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
MIRROR=https://mirror.scalewithus.com
for f in /etc/yum.repos.d/rocky*.repo; do
  [ -f "$f" ] || continue
  sudo sed -i \
    -e 's/^mirrorlist=/#mirrorlist=/' \
    -e 's/^metalink=/#metalink=/' \
    -e "s|^#[[:space:]]*baseurl=https\\?://dl\\.rockylinux\\.org/\\\$contentdir|baseurl=${MIRROR}/rocky|" \
    -e "s|^baseurl=https\\?://dl\\.rockylinux\\.org/\\\$contentdir|baseurl=${MIRROR}/rocky|" \
    "$f"
done
sudo dnf makecache
```

### Verify

```bash
curl -fsS https://mirror.scalewithus.com/healthz
sudo dnf makecache
sudo dnf repolist
```

### Revert

Restore `rocky*.repo` from `/var/backups/scalewithus-mirror-*/`, then `sudo dnf makecache`.
