# Rocky Linux 9 — cloud-init

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

## Verify

```bash
cloud-init status --wait
grep -R '^\[baseos\]\|^\[appstream\]\|^\[crb\]' /etc/yum.repos.d/
dnf repolist
dnf makecache
```
