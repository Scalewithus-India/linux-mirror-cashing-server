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

## Verify

```bash
cloud-init status --wait
apt-get update
```
