# Ubuntu — cloud-init

Mirror: `https://mirror.scalewithus.com`

## amd64

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

## arm64 / ports

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

Rewrite official Ubuntu hosts in `sources.list` and `sources.list.d` (`.list` and DEB822 `.sources`):

```bash
MIRROR=https://mirror.scalewithus.com
# traditional .list lines
sudo sed -i \
  -e "s|http://archive.ubuntu.com/ubuntu|${MIRROR}/ubuntu|g" \
  -e "s|https://archive.ubuntu.com/ubuntu|${MIRROR}/ubuntu|g" \
  -e "s|http://security.ubuntu.com/ubuntu|${MIRROR}/ubuntu-security|g" \
  -e "s|https://security.ubuntu.com/ubuntu|${MIRROR}/ubuntu-security|g" \
  -e "s|http://ports.ubuntu.com/ubuntu-ports|${MIRROR}/ubuntu-ports|g" \
  -e "s|https://ports.ubuntu.com/ubuntu-ports|${MIRROR}/ubuntu-ports|g" \
  /etc/apt/sources.list /etc/apt/sources.list.d/*.list 2>/dev/null || true

# DEB822 (.sources): replace URIs: lines the same way
sudo sed -i \
  -e "s|http://archive.ubuntu.com/ubuntu|${MIRROR}/ubuntu|g" \
  -e "s|https://archive.ubuntu.com/ubuntu|${MIRROR}/ubuntu|g" \
  -e "s|http://security.ubuntu.com/ubuntu|${MIRROR}/ubuntu-security|g" \
  -e "s|https://security.ubuntu.com/ubuntu|${MIRROR}/ubuntu-security|g" \
  -e "s|http://ports.ubuntu.com/ubuntu-ports|${MIRROR}/ubuntu-ports|g" \
  -e "s|https://ports.ubuntu.com/ubuntu-ports|${MIRROR}/ubuntu-ports|g" \
  /etc/apt/sources.list.d/*.sources 2>/dev/null || true

sudo apt-get update
```

**Ports note:** arm64 / armhf / etc. must use `/ubuntu-ports`, not `/ubuntu`.

### Verify

```bash
curl -fsS https://mirror.scalewithus.com/healthz
sudo apt-get update
```

### Revert

If you used the switch script, restore files from `/var/backups/scalewithus-mirror-*/` (paths mirror the originals), then `sudo apt-get update`.
