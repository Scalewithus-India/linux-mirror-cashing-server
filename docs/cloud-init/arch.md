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

## Verify (cloud-init)

```bash
cloud-init status --wait
pacman -Sy
```

## Live server

### One-liner

```bash
curl -fsSL https://mirror.scalewithus.com/switch-mirror.sh | sudo bash
```

### Manual

```bash
sudo tee /etc/pacman.d/mirrorlist >/dev/null <<'EOF'
Server = https://mirror.scalewithus.com/archlinux/$repo/os/$arch
EOF
sudo pacman -Sy
```

### Verify

```bash
curl -fsS https://mirror.scalewithus.com/healthz
sudo pacman -Sy
```

### Revert

Restore `/etc/pacman.d/mirrorlist` from `/var/backups/scalewithus-mirror-*/` (or your previous mirrorlist), then `sudo pacman -Sy`.
