# Cloud-init / client configs — mirror.scalewithus.com

Point package managers at `https://mirror.scalewithus.com` at first boot.

Wait for first boot before running package tools manually: `cloud-init status --wait`.

## Per OS

| OS | Doc |
|----|-----|
| Ubuntu (amd64 + arm64/ports) | [ubuntu.md](cloud-init/ubuntu.md) |
| Debian | [debian.md](cloud-init/debian.md) |
| AlmaLinux 9 | [almalinux.md](cloud-init/almalinux.md) |
| AlmaLinux 10 | [almalinux-10.md](cloud-init/almalinux-10.md) |
| Rocky Linux 9 | [rocky.md](cloud-init/rocky.md) |
| CentOS Stream 9 | [centos-stream.md](cloud-init/centos-stream.md) |
| Arch Linux | [arch.md](cloud-init/arch.md) |
| cPanel / WHM (FastUpdate) | [cpanel.md](cloud-init/cpanel.md) |

## Quick verify

```bash
cloud-init status --wait
curl -fsS https://mirror.scalewithus.com/healthz
```

Then use the verify steps in the OS-specific doc.
