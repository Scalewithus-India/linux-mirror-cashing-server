# AlmaLinux 9 — cloud-init

Mirror: `https://mirror.scalewithus.com`

For AlmaLinux 8 see [almalinux-8.md](almalinux-8.md); for 10 see [almalinux-10.md](almalinux-10.md).

Alma 9 ships **split** repo files (`almalinux-baseos.repo`, `almalinux-appstream.repo`, …). Overwrite those with `write_files` — do **not** create a combined `almalinux.repo` (duplicate IDs for `baseos` / `appstream` / `crb`).

Do **not** use cloud-init `yum_repos` with those same IDs.

```yaml
#cloud-config
write_files:
  - path: /etc/yum.repos.d/almalinux-baseos.repo
    permissions: "0644"
    content: |
      [baseos]
      name=AlmaLinux $releasever - BaseOS
      baseurl=https://mirror.scalewithus.com/almalinux/$releasever/BaseOS/$basearch/os/
      enabled=1
      gpgcheck=1
      countme=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9
      metadata_expire=86400
      enabled_metadata=1

      [baseos-debuginfo]
      name=AlmaLinux $releasever - BaseOS - Debug
      baseurl=https://vault.almalinux.org/$releasever/BaseOS/debug/$basearch/
      enabled=0
      gpgcheck=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9

      [baseos-source]
      name=AlmaLinux $releasever - BaseOS - Source
      baseurl=https://vault.almalinux.org/$releasever/BaseOS/Source/
      enabled=0
      gpgcheck=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9

  - path: /etc/yum.repos.d/almalinux-appstream.repo
    permissions: "0644"
    content: |
      [appstream]
      name=AlmaLinux $releasever - AppStream
      baseurl=https://mirror.scalewithus.com/almalinux/$releasever/AppStream/$basearch/os/
      enabled=1
      gpgcheck=1
      countme=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9
      metadata_expire=86400
      enabled_metadata=1

      [appstream-debuginfo]
      name=AlmaLinux $releasever - AppStream - Debug
      baseurl=https://vault.almalinux.org/$releasever/AppStream/debug/$basearch/
      enabled=0
      gpgcheck=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9

      [appstream-source]
      name=AlmaLinux $releasever - AppStream - Source
      baseurl=https://vault.almalinux.org/$releasever/AppStream/Source/
      enabled=0
      gpgcheck=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9

  - path: /etc/yum.repos.d/almalinux-extras.repo
    permissions: "0644"
    content: |
      [extras]
      name=AlmaLinux $releasever - Extras
      baseurl=https://mirror.scalewithus.com/almalinux/$releasever/extras/$basearch/os/
      enabled=1
      gpgcheck=1
      countme=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9
      metadata_expire=86400
      enabled_metadata=0

      [extras-debuginfo]
      name=AlmaLinux $releasever - Extras - Debug
      baseurl=https://vault.almalinux.org/$releasever/extras/debug/$basearch/
      enabled=0
      gpgcheck=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9

      [extras-source]
      name=AlmaLinux $releasever - Extras - Source
      baseurl=https://vault.almalinux.org/$releasever/extras/Source/
      enabled=0
      gpgcheck=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9

  - path: /etc/yum.repos.d/almalinux-crb.repo
    permissions: "0644"
    content: |
      [crb]
      name=AlmaLinux $releasever - CRB
      baseurl=https://mirror.scalewithus.com/almalinux/$releasever/CRB/$basearch/os/
      enabled=0
      gpgcheck=1
      countme=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9
      metadata_expire=86400
      enabled_metadata=0

      [crb-debuginfo]
      name=AlmaLinux $releasever - CRB - Debug
      baseurl=https://vault.almalinux.org/$releasever/CRB/debug/$basearch/
      enabled=0
      gpgcheck=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9

      [crb-source]
      name=AlmaLinux $releasever - CRB - Source
      baseurl=https://vault.almalinux.org/$releasever/CRB/Source/
      enabled=0
      gpgcheck=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9

  - path: /etc/yum.repos.d/almalinux-plus.repo
    permissions: "0644"
    content: |
      [plus]
      name=AlmaLinux $releasever - Plus
      baseurl=https://mirror.scalewithus.com/almalinux/$releasever/plus/$basearch/os/
      enabled=0
      gpgcheck=1
      countme=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9

      [plus-debuginfo]
      name=AlmaLinux $releasever - Plus - Debug
      baseurl=https://vault.almalinux.org/$releasever/plus/debug/$basearch/
      enabled=0
      gpgcheck=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9

      [plus-source]
      name=AlmaLinux $releasever - Plus - Source
      baseurl=https://vault.almalinux.org/$releasever/plus/Source/
      enabled=0
      gpgcheck=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9

  - path: /etc/yum.repos.d/almalinux-highavailability.repo
    permissions: "0644"
    content: |
      [highavailability]
      name=AlmaLinux $releasever - HighAvailability
      baseurl=https://mirror.scalewithus.com/almalinux/$releasever/HighAvailability/$basearch/os/
      enabled=0
      gpgcheck=1
      countme=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9

      [highavailability-debuginfo]
      name=AlmaLinux $releasever - HighAvailability - Debug
      baseurl=https://vault.almalinux.org/$releasever/HighAvailability/debug/$basearch/
      enabled=0
      gpgcheck=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9

      [highavailability-source]
      name=AlmaLinux $releasever - HighAvailability - Source
      baseurl=https://vault.almalinux.org/$releasever/HighAvailability/Source/
      enabled=0
      gpgcheck=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9

  - path: /etc/yum.repos.d/almalinux-nfv.repo
    permissions: "0644"
    content: |
      [nfv]
      name=AlmaLinux $releasever - NFV
      baseurl=https://mirror.scalewithus.com/almalinux/$releasever/NFV/$basearch/os/
      enabled=0
      gpgcheck=1
      countme=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9

      [nfv-debuginfo]
      name=AlmaLinux $releasever - NFV - Debug
      baseurl=https://vault.almalinux.org/$releasever/NFV/debug/$basearch/
      enabled=0
      gpgcheck=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9

      [nfv-source]
      name=AlmaLinux $releasever - NFV - Source
      baseurl=https://vault.almalinux.org/$releasever/NFV/Source/
      enabled=0
      gpgcheck=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9

  - path: /etc/yum.repos.d/almalinux-resilientstorage.repo
    permissions: "0644"
    content: |
      [resilientstorage]
      name=AlmaLinux $releasever - ResilientStorage
      baseurl=https://mirror.scalewithus.com/almalinux/$releasever/ResilientStorage/$basearch/os/
      enabled=0
      gpgcheck=1
      countme=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9

      [resilientstorage-debuginfo]
      name=AlmaLinux $releasever - ResilientStorage - Debug
      baseurl=https://vault.almalinux.org/$releasever/ResilientStorage/debug/$basearch/
      enabled=0
      gpgcheck=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9

      [resilientstorage-source]
      name=AlmaLinux $releasever - ResilientStorage - Source
      baseurl=https://vault.almalinux.org/$releasever/ResilientStorage/Source/
      enabled=0
      gpgcheck=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9

  - path: /etc/yum.repos.d/almalinux-rt.repo
    permissions: "0644"
    content: |
      [rt]
      name=AlmaLinux $releasever - RT
      baseurl=https://mirror.scalewithus.com/almalinux/$releasever/RT/$basearch/os/
      enabled=0
      gpgcheck=1
      countme=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9

      [rt-debuginfo]
      name=AlmaLinux $releasever - RT - Debug
      baseurl=https://vault.almalinux.org/$releasever/RT/debug/$basearch/
      enabled=0
      gpgcheck=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9

      [rt-source]
      name=AlmaLinux $releasever - RT - Source
      baseurl=https://vault.almalinux.org/$releasever/RT/Source/
      enabled=0
      gpgcheck=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9

  - path: /etc/yum.repos.d/almalinux-sap.repo
    permissions: "0644"
    content: |
      [sap]
      name=AlmaLinux $releasever - SAP
      baseurl=https://mirror.scalewithus.com/almalinux/$releasever/SAP/$basearch/os/
      enabled=0
      gpgcheck=1
      countme=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9

      [sap-debuginfo]
      name=AlmaLinux $releasever - SAP - Debug
      baseurl=https://vault.almalinux.org/$releasever/SAP/debug/$basearch/
      enabled=0
      gpgcheck=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9

      [sap-source]
      name=AlmaLinux $releasever - SAP - Source
      baseurl=https://vault.almalinux.org/$releasever/SAP/Source/
      enabled=0
      gpgcheck=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9

  - path: /etc/yum.repos.d/almalinux-saphana.repo
    permissions: "0644"
    content: |
      [saphana]
      name=AlmaLinux $releasever - SAPHANA
      baseurl=https://mirror.scalewithus.com/almalinux/$releasever/SAPHANA/$basearch/os/
      enabled=0
      gpgcheck=1
      countme=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9

      [saphana-debuginfo]
      name=AlmaLinux $releasever - SAPHANA - Debug
      baseurl=https://vault.almalinux.org/$releasever/SAPHANA/debug/$basearch/
      enabled=0
      gpgcheck=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9

      [saphana-source]
      name=AlmaLinux $releasever - SAPHANA - Source
      baseurl=https://vault.almalinux.org/$releasever/SAPHANA/Source/
      enabled=0
      gpgcheck=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9

  - path: /etc/yum.repos.d/epel.repo
    permissions: "0644"
    content: |
      [epel]
      name=Extra Packages for Enterprise Linux $releasever - $basearch
      baseurl=https://mirror.scalewithus.com/epel/$releasever/Everything/$basearch/
      enabled=1
      gpgcheck=1
      countme=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-EPEL-$releasever

runcmd:
  - rm -f /etc/yum.repos.d/almalinux.repo /etc/yum.repos.d/*.rpmnew /etc/yum.repos.d/*cloud_config* /etc/yum.repos.d/epel-cisco-openh264.repo
  - dnf -y install epel-release || true
  - |
      cat > /etc/yum.repos.d/epel.repo <<'EOF'
      [epel]
      name=Extra Packages for Enterprise Linux $releasever - $basearch
      baseurl=https://mirror.scalewithus.com/epel/$releasever/Everything/$basearch/
      enabled=1
      gpgcheck=1
      countme=1
      gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-EPEL-$releasever
      EOF
  - rm -f /etc/yum.repos.d/epel-cisco-openh264.repo /etc/yum.repos.d/*.rpmnew
  - dnf clean all
  - dnf -y makecache
```

## Live server

### One-liner

```bash
curl -fsSL https://mirror.scalewithus.com/switch-mirror.sh | sudo bash
# optional EPEL:
curl -fsSL https://mirror.scalewithus.com/switch-mirror.sh | sudo bash -s -- --epel
```

### Manual

Do **not** create a combined `almalinux.repo` — it duplicates `baseos` / `appstream` / `crb` against the stock split files.

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
grep -R '^\[baseos\]\|^\[appstream\]\|^\[crb\]' /etc/yum.repos.d/
# each ID must appear once
dnf repolist
dnf makecache
```

### Revert

Restore `almalinux-*.repo` from `/var/backups/scalewithus-mirror-*/`, then `dnf makecache`.

## Verify (cloud-init)

```bash
cloud-init status --wait
grep -R '^\[baseos\]\|^\[appstream\]\|^\[crb\]' /etc/yum.repos.d/
# each ID must appear once
dnf repolist
dnf makecache
```
