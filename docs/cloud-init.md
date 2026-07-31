# Cloud-init / client configs — mirror.scalewithus.com

## Live servers

Already running? Use the universal switcher:

```bash
curl -fsSL https://mirror.scalewithus.com/switch-mirror.sh | sudo bash
```

Full flags, safety notes, and links: [switch.md](cloud-init/switch.md). Each OS guide also has a **Live server** section (manual steps + one-liner).

## First boot (cloud-init)

Point package managers at `https://mirror.scalewithus.com` at first boot.

Wait for first boot before running package tools manually: `cloud-init status --wait`.

## Per OS

Ordered to match common product OS groups:

| OS | Doc |
|----|-----|
| Live switch (all Linux distros) | [switch.md](cloud-init/switch.md) |
| Windows | [windows.md](cloud-init/windows.md) (docs only — not a package mirror) |
| Ubuntu (amd64 + arm64/ports) | [ubuntu.md](cloud-init/ubuntu.md) |
| Alpine Linux | [alpine.md](cloud-init/alpine.md) |
| CentOS (Stream) | [centos-stream.md](cloud-init/centos-stream.md) |
| Debian | [debian.md](cloud-init/debian.md) |
| AlmaLinux 9 | [almalinux.md](cloud-init/almalinux.md) |
| AlmaLinux 10 | [almalinux-10.md](cloud-init/almalinux-10.md) |
| Rocky Linux 9 | [rocky.md](cloud-init/rocky.md) |
| Arch Linux | [arch.md](cloud-init/arch.md) |
| cPanel / WHM (FastUpdate) | [cpanel.md](cloud-init/cpanel.md) |

## Quick verify

```bash
curl -fsS https://mirror.scalewithus.com/healthz
# cloud-init hosts:
cloud-init status --wait
```

Then use the verify steps in the OS-specific doc.
