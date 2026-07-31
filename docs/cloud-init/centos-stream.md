# CentOS Stream 9 — cloud-init

Mirror: `https://mirror.scalewithus.com`

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

## Verify

```bash
cloud-init status --wait
grep -R '^\[baseos\]\|^\[appstream\]\|^\[crb\]' /etc/yum.repos.d/
dnf repolist
dnf makecache
```
