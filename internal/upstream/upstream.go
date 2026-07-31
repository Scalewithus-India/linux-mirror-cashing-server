package upstream

import (
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
)

type Mapping struct {
	Prefix string
	Base   string
}

var raw = []Mapping{
	{"/debian-security/", "https://security.debian.org/debian-security/"},
	{"/ubuntu-security/", "https://security.ubuntu.com/ubuntu/"},
	{"/ubuntu-ports/", "https://ports.ubuntu.com/ubuntu-ports/"},
	{"/ubuntu/", "https://archive.ubuntu.com/ubuntu/"},
	{"/debian/", "https://deb.debian.org/debian/"},
	{"/almalinux/", "https://repo.almalinux.org/almalinux/"},
	{"/rocky/", "https://dl.rockylinux.org/pub/rocky/"},
	{"/centos-stream/", "https://mirror.stream.centos.org/"},
	{"/archlinux/", "https://geo.mirror.pkgbuild.com/"},
	{"/alpine/", "https://dl-cdn.alpinelinux.org/alpine/"},
	{"/epel/", "https://download.fedoraproject.org/pub/epel/"},
	{"/cpanel/", "https://httpupdate.cpanel.net/"},
}

// Prefixes longest-first.
var Prefixes []Mapping

func init() {
	Prefixes = append([]Mapping(nil), raw...)
	sort.Slice(Prefixes, func(i, j int) bool {
		return len(Prefixes[i].Prefix) > len(Prefixes[j].Prefix)
	})
}

func Paths() []string {
	out := make([]string, len(Prefixes))
	for i, p := range Prefixes {
		out[i] = p.Prefix
	}
	return out
}

var metadataNameRE = regexp.MustCompile(`(?i)(InRelease|Release(\.gpg)?|Packages(\.(gz|xz|bz2|zst))?|` +
	`Sources(\.(gz|xz|bz2|zst))?|Contents-.*|repomd\.xml(\.asc)?|` +
	`lastupdate|lastsync|mirrorlist|md5sums|sha256sums|sha512sums|` +
	`TIERS\.json(\.asc)?|.*\.digest\.list(\.bz2)?|` +
	`APKINDEX\.tar\.gz|` +
	`index\.html|README)$`)

var allowedMetadataCTypes = map[string]struct{}{
	"application/octet-stream": {},
	"application/x-gzip":       {},
	"application/gzip":         {},
	"application/x-xz":         {},
	"application/zstd":         {},
	"application/x-bzip2":      {},
	"application/xml":          {},
	"text/xml":                 {},
	"text/plain":               {},
	"application/json":         {},
	"application/pgp-signature": {},
}

// NormalizePath collapses slashes and rejects traversal. Empty string means reject.
func NormalizePath(p string) string {
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if strings.Contains(p, "\x00") || strings.Contains(p, `\`) {
		return ""
	}
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	decoded, err := url.PathUnescape(p)
	if err != nil {
		return ""
	}
	if strings.Contains(decoded, "\x00") || strings.Contains(decoded, `\`) {
		return ""
	}
	trailing := strings.HasSuffix(decoded, "/") && decoded != "/"
	parts := make([]string, 0)
	for _, part := range strings.Split(decoded, "/") {
		if part == "" {
			continue
		}
		if part == "." || part == ".." || strings.Contains(part, `\`) || strings.Contains(part, "\x00") {
			return ""
		}
		parts = append(parts, part)
	}
	clean := "/" + strings.Join(parts, "/")
	if trailing {
		clean += "/"
	}
	return clean
}

type Resolved struct {
	Prefix      string
	Key         string // S3 key (no leading slash)
	UpstreamURL string
}

func Resolve(pathStr string) *Resolved {
	pathStr = NormalizePath(pathStr)
	if pathStr == "" {
		return nil
	}
	for _, m := range Prefixes {
		prefixTrim := strings.TrimRight(m.Prefix, "/")
		if pathStr == prefixTrim || strings.HasPrefix(pathStr, m.Prefix) {
			rel := ""
			if strings.HasPrefix(pathStr, m.Prefix) {
				rel = pathStr[len(m.Prefix):]
			}
			upstream := m.Base + escapePath(rel)
			key := strings.TrimPrefix(pathStr, "/")
			if strings.HasSuffix(pathStr, "/") || pathStr == prefixTrim {
				if !strings.HasSuffix(key, "/") {
					key += "/"
				}
			}
			return &Resolved{Prefix: m.Prefix, Key: key, UpstreamURL: upstream}
		}
	}
	return nil
}

// escapePath encodes path segments like Python quote(rel, safe="/-._~+=,").
func escapePath(rel string) string {
	if rel == "" {
		return ""
	}
	parts := strings.Split(rel, "/")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = url.PathEscape(p)
		// PathEscape encodes more than we want; restore safe chars.
		parts[i] = strings.NewReplacer(
			"%2D", "-",
			"%2E", ".",
			"%5F", "_",
			"%7E", "~",
			"%2B", "+",
			"%3D", "=",
			"%2C", ",",
		).Replace(parts[i])
	}
	return strings.Join(parts, "/")
}

func IsMetadataKey(key string) bool {
	name := path.Base(strings.TrimRight(key, "/"))
	if metadataNameRE.MatchString(name) {
		return true
	}
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".xml") ||
		strings.HasSuffix(lower, ".db") ||
		strings.HasSuffix(lower, ".db.sig") ||
		strings.HasSuffix(lower, ".files") ||
		strings.HasSuffix(lower, ".files.sig")
}

func ResponseContentType(key, upstream string) string {
	if !IsMetadataKey(key) {
		return "application/octet-stream"
	}
	if upstream == "" {
		return "application/octet-stream"
	}
	base := strings.ToLower(strings.TrimSpace(strings.SplitN(upstream, ";", 2)[0]))
	if base == "text/html" {
		return "application/octet-stream"
	}
	if _, ok := allowedMetadataCTypes[base]; !ok {
		return "application/octet-stream"
	}
	return strings.TrimSpace(strings.SplitN(upstream, ";", 2)[0])
}
