# Cloud-init configs — mirror.scalewithus.com

Point package managers at `https://mirror.scalewithus.com` at first boot.

> **AlmaLinux 9:** Repos are split across `almalinux-baseos.repo`, `almalinux-appstream.repo`, `almalinux-crb.repo`, etc. Do **not** add a second `almalinux.repo` (or cloud-init `yum_repos`) with the same IDs — that causes `Repository baseos/appstream/crb is listed more than once`. Patch the stock files instead.

> Wait for first boot to finish before running `dnf`/`yum` manually: `cloud-init status --wait`

---

## Ubuntu (amd64)

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

---

## Ubuntu (arm64 / ports)

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

---

## Debian

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

Force `sources.list` if needed (change `bookworm` for your release):

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

---

## AlmaLinux 9

```yaml
#cloud-config
runcmd:
  - |
      set -e
      # Remove mistaken combined / cloud-init duplicate repo files
      rm -f /etc/yum.repos.d/almalinux.repo \
            /etc/yum.repos.d/*cloud_config* \
            /etc/yum.repos.d/cloud-init*

      # Patch stock split repos (almalinux-baseos.repo, -appstream.repo, -crb.repo, …)
      for f in /etc/yum.repos.d/almalinux-*.repo; do
        [ -f "$f" ] || continue
        sed -i \
          -e 's/^mirrorlist=/#mirrorlist=/' \
          -e 's/^metalink=/#metalink=/' \
          -e 's|^#[[:space:]]*baseurl=https://repo\.almalinux\.org/almalinux|baseurl=https://mirror.scalewithus.com/almalinux|' \
          -e 's|^baseurl=https://repo\.almalinux\.org/almalinux|baseurl=https://mirror.scalewithus.com/almalinux|' \
          "$f"
      done

      # Optional: EPEL via this mirror
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

### Fix an already-booted Alma VM

```bash
cloud-init status --wait || true

# See which files define the same IDs
grep -R '^\[baseos\]\|^\[appstream\]\|^\[crb\]' /etc/yum.repos.d/

# Remove duplicates from bad cloud-init
rm -f /etc/yum.repos.d/almalinux.repo \
      /etc/yum.repos.d/*cloud_config* \
      /etc/yum.repos.d/cloud-init*

# Point stock split repos at the mirror
for f in /etc/yum.repos.d/almalinux-*.repo; do
  sed -i \
    -e 's/^mirrorlist=/#mirrorlist=/' \
    -e 's/^metalink=/#metalink=/' \
    -e 's|^#[[:space:]]*baseurl=https://repo\.almalinux\.org/almalinux|baseurl=https://mirror.scalewithus.com/almalinux|' \
    -e 's|^baseurl=https://repo\.almalinux\.org/almalinux|baseurl=https://mirror.scalewithus.com/almalinux|' \
    "$f"
done

# Optional EPEL
dnf -y install epel-release
rm -f /etc/yum.repos.d/epel-cisco-openh264.repo
for f in /etc/yum.repos.d/epel*.repo; do
  sed -i \
    -e 's/^metalink=/#metalink=/' \
    -e 's/^mirrorlist=/#mirrorlist=/' \
    -e 's|^#[[:space:]]*baseurl=https://download\.fedoraproject\.org/pub/epel|baseurl=https://mirror.scalewithus.com/epel|' \
    -e 's|^baseurl=https://download\.fedoraproject\.org/pub/epel|baseurl=https://mirror.scalewithus.com/epel|' \
    "$f"
done

dnf clean all
dnf makecache
dnf -y update
```

---

## Rocky Linux 9

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

---

## CentOS Stream 9

`$releasever` is usually `9-stream` (or `10-stream`).

```yaml
#cloud-config
runcmd:
  - |
      set -e
      rm -f /etc/yum.repos.d/*cloud_config* /etc/yum.repos.d/cloud-init*

      for f in /etc/yum.repos.d/centos*.repo; do
        [ -f "$f" ] || continue
        sed -i \
          -e 's/^metalink=/#metalink=/' \
          -e 's/^mirrorlist=/#mirrorlist=/' \
          -e 's|^#[[:space:]]*baseurl=https://mirror\.stream\.centos\.org|baseurl=https://mirror.scalewithus.com/centos-stream|' \
          -e 's|^baseurl=https://mirror\.stream\.centos\.org|baseurl=https://mirror.scalewithus.com/centos-stream|' \
          "$f"
      done

      # If stock files only had metalink (no baseurl), write explicit repos
      if ! grep -q '^baseurl=https://mirror.scalewithus.com/centos-stream' /etc/yum.repos.d/centos*.repo 2>/dev/null; then
        cat > /etc/yum.repos.d/centos-swu.repo <<'EOF'
      [baseos]
      name=CentOS Stream $releasever - BaseOS
      baseurl=https://mirror.scalewithus.com/centos-stream/$releasever/BaseOS/$basearch/os/
      enabled=1
      gpgcheck=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-centosofficial

      [appstream]
      name=CentOS Stream $releasever - AppStream
      baseurl=https://mirror.scalewithus.com/centos-stream/$releasever/AppStream/$basearch/os/
      enabled=1
      gpgcheck=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-centosofficial

      [crb]
      name=CentOS Stream $releasever - CRB
      baseurl=https://mirror.scalewithus.com/centos-stream/$releasever/CRB/$basearch/os/
      enabled=1
      gpgcheck=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-centosofficial
      EOF
        # Disable stock definitions of the same IDs
        sed -i 's/^enabled=1/enabled=0/' /etc/yum.repos.d/centos.repo /etc/yum.repos.d/centos-addons.repo 2>/dev/null || true
      fi

      dnf -y install epel-release || true
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

---

## Arch Linux

```yaml
#cloud-config
write_files:
  - path: /etc/pacman.d/mirrorlist
    permissions: "0644"
    content: |
      Server = https://mirror.scalewithus.com/archlinux/$repo/os/$arch
runcmd:
  - [pacman, -Sy, --noconfirm]
```

---

## cPanel / WHM (FastUpdate)

cPanel’s `HTTPUPDATE` must be a **hostname** (files are fetched from `/cpanelsync/…`, `/RPM/…` at the site root). Use a root vhost (not a `/cpanel` path).

### Option A — dedicated hostname

DNS: `httpupdate.scalewithus.com` → same IP as the mirror (port **80**; HTTPS also works on this name).

```text
# /etc/cpsources.conf
HTTPUPDATE=httpupdate.scalewithus.com
```

### Option B — hosts entry for official name (no cpsources change)

On the cPanel server, force the official update host to this mirror (HTTP only):

```text
# /etc/hosts
103.249.112.156  httpupdate.cpanel.net
```

Leave default `HTTPUPDATE` alone (or omit `/etc/cpsources.conf`). upcp will request `http://httpupdate.cpanel.net/...` and hit this mirror.

```bash
# verify hosts override
getent hosts httpupdate.cpanel.net
curl -fsSI http://httpupdate.cpanel.net/cpanelsync/
curl -fsSI http://httpupdate.cpanel.net/RPM/
/scripts/upcp
```

Do **not** rely on `https://httpupdate.cpanel.net` via hosts — TLS will not match (we cannot issue a cert for `cpanel.net`).

### Debug path on the main mirror host

```text
https://mirror.scalewithus.com/cpanel/cpanelsync/
```

Do **not** put a path in `HTTPUPDATE` (e.g. `mirror.scalewithus.com/cpanel`) — upcp will request `http://host/cpanelsync/…` at the root.

---

## Quick verify after boot

```bash
cloud-init status --wait
curl -fsS https://mirror.scalewithus.com/healthz
# Ubuntu/Debian
apt-get update
# Alma/Rocky/Stream — must show each ID only once
grep -R '^\[baseos\]\|^\[appstream\]\|^\[crb\]' /etc/yum.repos.d/
dnf repolist
dnf makecache
# Arch
pacman -Sy
```
