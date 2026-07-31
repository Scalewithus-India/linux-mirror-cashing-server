# Arch Linux — cloud-init

Mirror: `https://mirror.scalewithus.com`

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

## Verify

```bash
cloud-init status --wait
pacman -Sy
```
