# Live server — switch to this mirror

Already-running hosts can point package managers at `https://mirror.scalewithus.com` without reinstalling.

## Universal one-liner

Requires **root**. Backs up touched files under `/var/backups/scalewithus-mirror-YYYYMMDD-HHMMSS/` before writing.

```bash
curl -fsSL https://mirror.scalewithus.com/switch-mirror.sh | sudo bash
```

Optional flags:

```bash
# Preview only
curl -fsSL https://mirror.scalewithus.com/switch-mirror.sh | sudo bash -s -- --dry-run

# Alma / Rocky / CentOS Stream: also install + point EPEL
curl -fsSL https://mirror.scalewithus.com/switch-mirror.sh | sudo bash -s -- --epel

# cPanel: append httpupdate.cpanel.net → mirror IP in /etc/hosts
curl -fsSL https://mirror.scalewithus.com/switch-mirror.sh | sudo bash -s -- --cpanel-hosts

# Skip apt-get update / dnf makecache / pacman -Sy / apk update
curl -fsSL https://mirror.scalewithus.com/switch-mirror.sh | sudo bash -s -- --no-makecache

# Override mirror hostname
curl -fsSL https://mirror.scalewithus.com/switch-mirror.sh | sudo MIRROR_HOST=mirror.scalewithus.com bash -s -- --dry-run
```

Script URL: [https://mirror.scalewithus.com/switch-mirror.sh](https://mirror.scalewithus.com/switch-mirror.sh)

## Supported distros

| Distro | What the script changes |
|--------|-------------------------|
| Ubuntu | `/etc/apt/sources.list` + `sources.list.d` (`.list` and DEB822 `.sources`); ports → `/ubuntu-ports` |
| Debian | same for Debian / Debian-security URIs |
| AlmaLinux | removes accidental `almalinux.repo`; sed on `almalinux-*.repo` |
| CloudLinux | same Alma baseurl rewrite (CLN is Alma/RHEL underneath); native `cloudlinux*.repo` left alone |
| Rocky Linux | sed on `rocky*.repo` |
| CentOS (Stream) | metalink/mirrorlist off; baseurl → `/centos-stream/` (or writes `centos-swu.repo`) |
| Alpine Linux | rewrite Alpine CDN hosts in `/etc/apk/repositories` → `/alpine/` |
| Arch Linux | `/etc/pacman.d/mirrorlist` |

cPanel FastUpdate: auto-enabled when `/usr/local/cpanel` (or `cpsources.conf`) is present; or force with `--cpanel-hosts`. See [cpanel.md](cpanel.md).

Windows guests: this script does not apply — see [windows.md](windows.md).

## Safety

- Refuses non-root
- Always backups before write
- Does not disable GPG checks
- Only installs a package when `--epel` (then `epel-release`)
- HTTPS mirror URLs only

## Manual steps per OS

Use the **Live server** section in each guide:

- [Ubuntu](ubuntu.md#live-server)
- [Debian](debian.md#live-server)
- [Alpine](alpine.md#live-server)
- [AlmaLinux 8](almalinux-8.md#live-server) · [9](almalinux.md#live-server) · [10](almalinux-10.md#live-server)
- [Rocky Linux 8](rocky-8.md#live-server) · [9](rocky.md#live-server) · [10](rocky-10.md#live-server)
- [CentOS (Stream)](centos-stream.md#live-server)
- [Arch](arch.md#live-server)
- [cPanel](cpanel.md#live-server)
- [Windows](windows.md) (docs only — no package switcher)

## Verify

```bash
curl -fsS https://mirror.scalewithus.com/healthz
# then apt-get update / dnf makecache / pacman -Sy / apk update as appropriate
```

## Revert

Restore files from the backup directory printed by the script (paths under the backup mirror the original absolute paths), then refresh the package manager cache.
