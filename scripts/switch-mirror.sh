#!/usr/bin/env bash
# ScaleWithUs Linux Mirror — switch a live server to https://mirror.scalewithus.com
# Usage:
#   curl -fsSL https://mirror.scalewithus.com/switch-mirror.sh | sudo bash
#   curl -fsSL ... | sudo bash -s -- --dry-run
#   curl -fsSL ... | sudo bash -s -- --epel
#   curl -fsSL ... | sudo MIRROR_HOST=mirror.example.com bash -s -- --cpanel-hosts
set -euo pipefail

MIRROR_HOST="${MIRROR_HOST:-mirror.scalewithus.com}"
MIRROR="https://${MIRROR_HOST}"
DRY_RUN=0
DO_EPEL=0
DO_CPANEL_HOSTS=0
NO_MAKECACHE=0
BACKUP_DIR=""

usage() {
  cat <<EOF
Usage: sudo bash switch-mirror.sh [options]

  --dry-run         Print actions only; do not write files
  --epel            EL: ensure EPEL and point at ${MIRROR}/epel/
  --cpanel-hosts    Append httpupdate.cpanel.net → mirror IP to /etc/hosts
  --no-makecache    Skip apt/dnf/pacman/apk refresh at the end
  -h, --help        Show this help

Environment:
  MIRROR_HOST       Default: mirror.scalewithus.com
EOF
}

log()  { printf '[switch-mirror] %s\n' "$*"; }
warn() { printf '[switch-mirror] WARN: %s\n' "$*" >&2; }
die()  { printf '[switch-mirror] ERROR: %s\n' "$*" >&2; exit 1; }

run() {
  if [[ "$DRY_RUN" -eq 1 ]]; then
    log "DRY-RUN: $*"
  else
    "$@"
  fi
}

backup_file() {
  local src="$1"
  [[ -e "$src" ]] || return 0
  local dest="${BACKUP_DIR}${src}"
  if [[ "$DRY_RUN" -eq 1 ]]; then
    log "DRY-RUN: backup $src -> $dest"
    return 0
  fi
  mkdir -p "$(dirname "$dest")"
  cp -a "$src" "$dest"
}

write_file() {
  local path="$1"
  local content="$2"
  backup_file "$path"
  if [[ "$DRY_RUN" -eq 1 ]]; then
    log "DRY-RUN: write $path"
    return 0
  fi
  mkdir -p "$(dirname "$path")"
  printf '%s\n' "$content" >"$path"
}

rewrite_inplace() {
  # rewrite_inplace <path> <sed_script...>
  local path="$1"
  shift
  [[ -f "$path" ]] || return 0
  backup_file "$path"
  if [[ "$DRY_RUN" -eq 1 ]]; then
    log "DRY-RUN: sed $path"
    return 0
  fi
  sed -i "$@" "$path"
}

resolve_mirror_ip() {
  local ip
  ip="$(getent ahostsv4 "$MIRROR_HOST" 2>/dev/null | awk '{print $1; exit}')"
  if [[ -z "$ip" ]]; then
    ip="$(getent hosts "$MIRROR_HOST" 2>/dev/null | awk '{print $1; exit}')"
  fi
  [[ -n "$ip" ]] || die "Could not resolve $MIRROR_HOST"
  printf '%s\n' "$ip"
}

# --- Ubuntu / Debian apt ----------------------------------------------------

rewrite_apt_line() {
  # stdin -> stdout: rewrite deb/deb-src lines for Ubuntu or Debian
  local family="$1"  # ubuntu|debian
  local line host path rest
  while IFS= read -r line || [[ -n "$line" ]]; do
    if [[ "$line" =~ ^(deb(-src)?[[:space:]]+)(https?://[^[:space:]]+)(.*)$ ]]; then
      local prefix="${BASH_REMATCH[1]}"
      local url="${BASH_REMATCH[3]}"
      rest="${BASH_REMATCH[4]}"
      host="${url#*://}"
      path="/${host#*/}"
      host="${host%%/*}"
      path="${path%/}"
      case "$family" in
        ubuntu)
          case "$host" in
            archive.ubuntu.com|security.ubuntu.com)
              if [[ "$host" == "security.ubuntu.com" ]]; then
                printf '%s%s/ubuntu-security%s\n' "$prefix" "$MIRROR" "$rest"
              else
                printf '%s%s/ubuntu%s\n' "$prefix" "$MIRROR" "$rest"
              fi
              ;;
            ports.ubuntu.com)
              printf '%s%s/ubuntu-ports%s\n' "$prefix" "$MIRROR" "$rest"
              ;;
            *)
              # Already our mirror or other — leave unless pointing at known ubuntu paths under our host
              printf '%s\n' "$line"
              ;;
          esac
          ;;
        debian)
          case "$host" in
            deb.debian.org|ftp.debian.org|cdn-fastly.deb.debian.org)
              printf '%s%s/debian%s\n' "$prefix" "$MIRROR" "$rest"
              ;;
            security.debian.org)
              printf '%s%s/debian-security%s\n' "$prefix" "$MIRROR" "$rest"
              ;;
            *)
              printf '%s\n' "$line"
              ;;
          esac
          ;;
        *)
          printf '%s\n' "$line"
          ;;
      esac
    else
      printf '%s\n' "$line"
    fi
  done
}

rewrite_deb822_file() {
  local path="$1"
  local family="$2"
  [[ -f "$path" ]] || return 0
  backup_file "$path"
  if [[ "$DRY_RUN" -eq 1 ]]; then
    log "DRY-RUN: rewrite DEB822 $path"
    return 0
  fi
  local tmp
  tmp="$(mktemp)"
  local line uri
  while IFS= read -r line || [[ -n "$line" ]]; do
    if [[ "$line" =~ ^([Uu][Rr][Ii][s]?[[:space:]]*:[[:space:]]*)(.+)$ ]]; then
      local prefix="${BASH_REMATCH[1]}"
      uri="${BASH_REMATCH[2]}"
      uri="$(printf '%s\n' "$uri" | tr -d '\r')"
      case "$family" in
        ubuntu)
          uri="${uri//https:\/\/archive.ubuntu.com\/ubuntu/${MIRROR}/ubuntu}"
          uri="${uri//http:\/\/archive.ubuntu.com\/ubuntu/${MIRROR}/ubuntu}"
          uri="${uri//https:\/\/security.ubuntu.com\/ubuntu/${MIRROR}/ubuntu-security}"
          uri="${uri//http:\/\/security.ubuntu.com\/ubuntu/${MIRROR}/ubuntu-security}"
          uri="${uri//https:\/\/ports.ubuntu.com\/ubuntu-ports/${MIRROR}/ubuntu-ports}"
          uri="${uri//http:\/\/ports.ubuntu.com\/ubuntu-ports/${MIRROR}/ubuntu-ports}"
          ;;
        debian)
          uri="${uri//https:\/\/deb.debian.org\/debian/${MIRROR}/debian}"
          uri="${uri//http:\/\/deb.debian.org\/debian/${MIRROR}/debian}"
          uri="${uri//https:\/\/security.debian.org\/debian-security/${MIRROR}/debian-security}"
          uri="${uri//http:\/\/security.debian.org\/debian-security/${MIRROR}/debian-security}"
          uri="${uri//https:\/\/security.debian.org\/debian/${MIRROR}/debian-security}"
          ;;
      esac
      printf '%s%s\n' "$prefix" "$uri"
    else
      printf '%s\n' "$line"
    fi
  done <"$path" >"$tmp"
  mv "$tmp" "$path"
}

handle_apt() {
  local family="$1"  # ubuntu|debian
  log "Configuring apt ($family) → $MIRROR"

  local f
  if [[ -f /etc/apt/sources.list ]]; then
    backup_file /etc/apt/sources.list
    if [[ "$DRY_RUN" -eq 1 ]]; then
      log "DRY-RUN: rewrite /etc/apt/sources.list"
    else
      local tmp
      tmp="$(mktemp)"
      rewrite_apt_line "$family" </etc/apt/sources.list >"$tmp"
      mv "$tmp" /etc/apt/sources.list
    fi
  fi

  shopt -s nullglob
  for f in /etc/apt/sources.list.d/*.list; do
    backup_file "$f"
    if [[ "$DRY_RUN" -eq 1 ]]; then
      log "DRY-RUN: rewrite $f"
    else
      local tmp
      tmp="$(mktemp)"
      rewrite_apt_line "$family" <"$f" >"$tmp"
      mv "$tmp" "$f"
    fi
  done
  for f in /etc/apt/sources.list.d/*.sources; do
    rewrite_deb822_file "$f" "$family"
  done
  shopt -u nullglob

  if [[ "$NO_MAKECACHE" -eq 0 ]]; then
    log "Running apt-get update"
    run apt-get update
  fi
}

# --- AlmaLinux / CloudLinux (Alma base) -------------------------------------

patch_almalinux_repos() {
  # Shared by AlmaLinux and CloudLinux (CLN is Alma/RHEL underneath for base OS).
  if [[ -f /etc/yum.repos.d/almalinux.repo ]]; then
    log "Removing combined /etc/yum.repos.d/almalinux.repo (use split almalinux-*.repo)"
    backup_file /etc/yum.repos.d/almalinux.repo
    run rm -f /etc/yum.repos.d/almalinux.repo
  fi

  shopt -s nullglob
  local f any=0
  for f in /etc/yum.repos.d/almalinux-*.repo /etc/yum.repos.d/almalinux.repo; do
    [[ -f "$f" ]] || continue
    any=1
    backup_file "$f"
    if [[ "$DRY_RUN" -eq 1 ]]; then
      log "DRY-RUN: sed $f"
      continue
    fi
    sed -i \
      -e 's|^mirrorlist=|#mirrorlist=|g' \
      -e 's|^metalink=|#metalink=|g' \
      -e 's|^# *baseurl=https\?://repo\.almalinux\.org/almalinux|baseurl='"${MIRROR}"'/almalinux|g' \
      -e 's|^baseurl=https\?://repo\.almalinux\.org/almalinux|baseurl='"${MIRROR}"'/almalinux|g' \
      -e 's|^# *baseurl=https\?://[a-z0-9.-]*\.almalinux\.org/almalinux|baseurl='"${MIRROR}"'/almalinux|g' \
      -e 's|^baseurl=https\?://[a-z0-9.-]*\.almalinux\.org/almalinux|baseurl='"${MIRROR}"'/almalinux|g' \
      "$f"
  done
  for f in /etc/yum.repos.d/*.rpmnew; do
    log "Removing leftover $f"
    run rm -f "$f"
  done
  shopt -u nullglob
  if [[ "$any" -eq 0 ]]; then
    warn "No almalinux-*.repo files found under /etc/yum.repos.d (CloudLinux native repos are left unchanged)"
  fi
}

handle_almalinux() {
  log "Configuring AlmaLinux → $MIRROR/almalinux/"
  patch_almalinux_repos
  handle_epel_optional
  if [[ "$NO_MAKECACHE" -eq 0 ]]; then
    log "Running dnf makecache"
    run dnf makecache
  fi
}

handle_cloudlinux() {
  # CloudLinux 8/9: Alma-compatible base repos + cPanel FastUpdate when WHM is installed.
  log "Configuring CloudLinux (Alma base repos) → $MIRROR/almalinux/"
  patch_almalinux_repos
  handle_epel_optional
  if [[ "$NO_MAKECACHE" -eq 0 ]]; then
    log "Running dnf makecache"
    run dnf makecache
  fi
}

cpanel_present() {
  [[ -d /usr/local/cpanel ]] || [[ -f /etc/cpsources.conf ]] || [[ -x /usr/local/cpanel/cpanel ]]
}

# --- Rocky ------------------------------------------------------------------

handle_rocky() {
  log "Configuring Rocky Linux → $MIRROR/rocky/"

  shopt -s nullglob
  local f
  for f in /etc/yum.repos.d/rocky*.repo; do
    backup_file "$f"
    if [[ "$DRY_RUN" -eq 1 ]]; then
      log "DRY-RUN: sed $f"
      continue
    fi
    sed -i \
      -e 's|^mirrorlist=|#mirrorlist=|g' \
      -e 's|^#baseurl=http://dl.rockylinux.org/\$contentdir|baseurl='"${MIRROR}"'/rocky|g' \
      -e 's|^baseurl=http://dl.rockylinux.org/\$contentdir|baseurl='"${MIRROR}"'/rocky|g' \
      -e 's|^#baseurl=https://dl.rockylinux.org/\$contentdir|baseurl='"${MIRROR}"'/rocky|g' \
      -e 's|^baseurl=https://dl.rockylinux.org/\$contentdir|baseurl='"${MIRROR}"'/rocky|g' \
      "$f"
  done
  shopt -u nullglob

  handle_epel_optional
  if [[ "$NO_MAKECACHE" -eq 0 ]]; then
    log "Running dnf makecache"
    run dnf makecache
  fi
}

# --- CentOS Stream ----------------------------------------------------------

handle_centos_stream() {
  log "Configuring CentOS Stream → $MIRROR/centos-stream/"

  shopt -s nullglob
  local f found=0
  for f in /etc/yum.repos.d/centos*.repo /etc/yum.repos.d/CentOS*.repo; do
    [[ -f "$f" ]] || continue
    found=1
    backup_file "$f"
    if [[ "$DRY_RUN" -eq 1 ]]; then
      log "DRY-RUN: sed $f"
      continue
    fi
    sed -i \
      -e 's|^metalink=|#metalink=|g' \
      -e 's|^mirrorlist=|#mirrorlist=|g' \
      -e 's|^# *baseurl=https\?://mirror\.stream\.centos\.org|baseurl='"${MIRROR}"'/centos-stream|g' \
      -e 's|^baseurl=https\?://mirror\.stream\.centos\.org|baseurl='"${MIRROR}"'/centos-stream|g' \
      "$f"
  done
  shopt -u nullglob

  if [[ "$found" -eq 0 ]]; then
    log "No centos*.repo found; writing /etc/yum.repos.d/centos-swu.repo"
    write_file /etc/yum.repos.d/centos-swu.repo \
"[baseos]
name=CentOS Stream \$releasever - BaseOS (ScaleWithUs)
baseurl=${MIRROR}/centos-stream/\$releasever/BaseOS/\$basearch/os/
gpgcheck=1
enabled=1
gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-centosofficial

[appstream]
name=CentOS Stream \$releasever - AppStream (ScaleWithUs)
baseurl=${MIRROR}/centos-stream/\$releasever/AppStream/\$basearch/os/
gpgcheck=1
enabled=1
gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-centosofficial

[crb]
name=CentOS Stream \$releasever - CRB (ScaleWithUs)
baseurl=${MIRROR}/centos-stream/\$releasever/CRB/\$basearch/os/
gpgcheck=1
enabled=1
gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-centosofficial"
  fi

  handle_epel_optional
  if [[ "$NO_MAKECACHE" -eq 0 ]]; then
    log "Running dnf makecache"
    run dnf makecache
  fi
}

# --- EPEL (optional) --------------------------------------------------------

handle_epel_optional() {
  [[ "$DO_EPEL" -eq 1 ]] || return 0
  log "Configuring EPEL → $MIRROR/epel/"

  if ! rpm -q epel-release &>/dev/null; then
    log "Installing epel-release"
    run dnf install -y epel-release
  fi

  shopt -s nullglob
  local f
  for f in /etc/yum.repos.d/epel*.repo; do
    backup_file "$f"
    if [[ "$DRY_RUN" -eq 1 ]]; then
      log "DRY-RUN: sed $f"
      continue
    fi
    sed -i \
      -e 's|^metalink=|#metalink=|g' \
      -e 's|^mirrorlist=|#mirrorlist=|g' \
      -e 's|^# *baseurl=https\?://download\.fedoraproject\.org/pub/epel|baseurl='"${MIRROR}"'/epel|g' \
      -e 's|^baseurl=https\?://download\.fedoraproject\.org/pub/epel|baseurl='"${MIRROR}"'/epel|g' \
      "$f"
  done
  shopt -u nullglob
}

# --- Arch -------------------------------------------------------------------

handle_arch() {
  log "Configuring Arch → $MIRROR/archlinux/"
  write_file /etc/pacman.d/mirrorlist \
"Server = ${MIRROR}/archlinux/\$repo/os/\$arch"
  if [[ "$NO_MAKECACHE" -eq 0 ]]; then
    log "Running pacman -Sy"
    run pacman -Sy --noconfirm
  fi
}

# --- Alpine -----------------------------------------------------------------

handle_alpine() {
  log "Configuring Alpine → $MIRROR/alpine/"
  local repos=/etc/apk/repositories
  [[ -f "$repos" ]] || die "Missing $repos"
  backup_file "$repos"
  if [[ "$DRY_RUN" -eq 1 ]]; then
    log "DRY-RUN: rewrite Alpine CDN hosts in $repos"
  else
    # Keep version/path (v3.21/main, edge/community, …); only change host → our /alpine/
    sed -i -E \
      -e "s|https?://[^[:space:]]+/alpine|${MIRROR}/alpine|g" \
      "$repos"
  fi
  if [[ "$NO_MAKECACHE" -eq 0 ]]; then
    log "Running apk update"
    run apk update
  fi
}

# --- cPanel hosts -----------------------------------------------------------

handle_cpanel_hosts() {
  [[ "$DO_CPANEL_HOSTS" -eq 1 ]] || return 0
  local ip
  ip="$(resolve_mirror_ip)"
  log "Pointing httpupdate.cpanel.net → $ip in /etc/hosts"
  backup_file /etc/hosts
  if [[ "$DRY_RUN" -eq 1 ]]; then
    log "DRY-RUN: set $ip httpupdate.cpanel.net"
    return 0
  fi
  # Drop prior ScaleWithUs-managed lines, then set/replace the hosts entry
  sed -i '/# scalewithus-mirror-cpanel$/d' /etc/hosts 2>/dev/null || true
  sed -i '/[[:space:]]httpupdate\.cpanel\.net[[:space:]]*# scalewithus-mirror/d' /etc/hosts 2>/dev/null || true
  if grep -qE '^[[:space:]]*[0-9a-fA-F.:]+[[:space:]]+httpupdate\.cpanel\.net([[:space:]]|$)' /etc/hosts; then
    log "Updating existing httpupdate.cpanel.net hosts entry → $ip"
    sed -i -E "s|^[[:space:]]*[0-9a-fA-F.:]+[[:space:]]+httpupdate\.cpanel\.net([[:space:]].*)?$|${ip}  httpupdate.cpanel.net  # scalewithus-mirror|" /etc/hosts
  else
    printf '%s  httpupdate.cpanel.net  # scalewithus-mirror\n' "$ip" >>/etc/hosts
  fi
}

# --- main -------------------------------------------------------------------

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) DRY_RUN=1 ;;
    --epel) DO_EPEL=1 ;;
    --cpanel-hosts) DO_CPANEL_HOSTS=1 ;;
    --no-makecache) NO_MAKECACHE=1 ;;
    -h|--help) usage; exit 0 ;;
    *) die "Unknown option: $1 (try --help)" ;;
  esac
  shift
done

[[ "$(id -u)" -eq 0 ]] || die "Must run as root (use sudo)"

[[ -r /etc/os-release ]] || die "/etc/os-release not found"
# shellcheck disable=SC1091
. /etc/os-release

BACKUP_DIR="/var/backups/scalewithus-mirror-$(date +%Y%m%d-%H%M%S)"
if [[ "$DRY_RUN" -eq 0 ]]; then
  mkdir -p "$BACKUP_DIR"
else
  BACKUP_DIR="/var/backups/scalewithus-mirror-DRY-RUN"
fi

log "Mirror: $MIRROR"
log "Detected: ${NAME:-$ID} ${VERSION_ID:-} (ID=$ID ID_LIKE=${ID_LIKE:-})"
log "Backup dir: $BACKUP_DIR"

ID_LOWER="$(printf '%s' "${ID:-}" | tr '[:upper:]' '[:lower:]')"
LIKE_LOWER="$(printf '%s' "${ID_LIKE:-}" | tr '[:upper:]' '[:lower:]')"

case "$ID_LOWER" in
  ubuntu)
    handle_apt ubuntu
    ;;
  debian)
    handle_apt debian
    ;;
  almalinux)
    handle_almalinux
    ;;
  cloudlinux)
    handle_cloudlinux
    ;;
  rocky)
    handle_rocky
    ;;
  centos|centos-stream)
    handle_centos_stream
    ;;
  arch|archlinux)
    handle_arch
    ;;
  alpine)
    handle_alpine
    ;;
  *)
    # Fallbacks via ID_LIKE
    if [[ "$LIKE_LOWER" == *ubuntu* ]]; then
      handle_apt ubuntu
    elif [[ "$LIKE_LOWER" == *debian* ]]; then
      handle_apt debian
    elif [[ "$LIKE_LOWER" == *rhel* || "$LIKE_LOWER" == *fedora* ]]; then
      die "Unsupported EL variant '$ID'. Supported: almalinux, cloudlinux, rocky, centos (Stream). Use a per-OS guide or set repos manually."
    else
      die "Unsupported distro ID='$ID'. See https://${MIRROR_HOST}/guides/switch"
    fi
    ;;
esac

# Auto-enable cPanel FastUpdate hosts override when WHM/cPanel is installed
if [[ "$DO_CPANEL_HOSTS" -eq 0 ]] && cpanel_present; then
  log "cPanel/WHM detected; enabling httpupdate.cpanel.net → mirror IP"
  DO_CPANEL_HOSTS=1
fi
handle_cpanel_hosts

log "Done."
log "Verify: curl -fsS ${MIRROR}/healthz"
log "Revert: restore files from ${BACKUP_DIR}/ (paths mirror the original absolute paths)"
if [[ "$DO_CPANEL_HOSTS" -eq 1 ]]; then
  log "cPanel: use HTTP (not HTTPS) to httpupdate.cpanel.net; or set HTTPUPDATE=httpupdate.scalewithus.com"
fi
