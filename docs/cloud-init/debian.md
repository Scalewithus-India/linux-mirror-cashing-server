# Debian — cloud-init

Mirror: `https://mirror.scalewithus.com`

## apt module

```yaml
#cloud-config
apt:
  primary:
    - arches: [default]
      uri: https://mirror.scalewithus.com/debian
  security:
    - arches: [default]
      uri: https://mirror.scalewithus.com/debian-security
```

## Force `sources.list`

Change `bookworm` for your release (`trixie`, etc.):

```yaml
#cloud-config
write_files:
  - path: /etc/apt/sources.list
    permissions: "0644"
    content: |
      deb https://mirror.scalewithus.com/debian bookworm main contrib non-free non-free-firmware
      deb https://mirror.scalewithus.com/debian bookworm-updates main contrib non-free non-free-firmware
      deb https://mirror.scalewithus.com/debian-security bookworm-security main contrib non-free non-free-firmware
runcmd:
  - [apt-get, update]
```

## Verify (cloud-init)

```bash
cloud-init status --wait
apt-get update
```

## Live server

### One-liner

```bash
curl -fsSL https://mirror.scalewithus.com/switch-mirror.sh | sudo bash
```

### Manual

```bash
MIRROR=https://mirror.scalewithus.com
sudo sed -i \
  -e "s|http://deb.debian.org/debian|${MIRROR}/debian|g" \
  -e "s|https://deb.debian.org/debian|${MIRROR}/debian|g" \
  -e "s|http://security.debian.org/debian-security|${MIRROR}/debian-security|g" \
  -e "s|https://security.debian.org/debian-security|${MIRROR}/debian-security|g" \
  -e "s|http://security.debian.org/debian|${MIRROR}/debian-security|g" \
  -e "s|https://security.debian.org/debian|${MIRROR}/debian-security|g" \
  /etc/apt/sources.list /etc/apt/sources.list.d/*.list /etc/apt/sources.list.d/*.sources 2>/dev/null || true
sudo apt-get update
```

### Verify

```bash
curl -fsS https://mirror.scalewithus.com/healthz
sudo apt-get update
```

### Revert

Restore from `/var/backups/scalewithus-mirror-*/` if you used the switch script, then `sudo apt-get update`.
