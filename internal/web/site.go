package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	gmhtml "github.com/yuin/goldmark/renderer/html"

	"github.com/Scalewithus-India/linux-mirror-cashing-server/internal/config"
	"github.com/Scalewithus-India/linux-mirror-cashing-server/internal/metrics"
	"github.com/Scalewithus-India/linux-mirror-cashing-server/internal/mirror"
	"github.com/Scalewithus-India/linux-mirror-cashing-server/internal/store"
	"github.com/Scalewithus-India/linux-mirror-cashing-server/internal/upstream"
)

const (
	githubRepo     = "Scalewithus-India/linux-mirror-cashing-server"
	githubURL      = "https://github.com/" + githubRepo
	githubAPIURL   = "https://api.github.com/repos/" + githubRepo
	githubStarsTTL = 600 * time.Second
)

type Guide struct {
	Slug  string
	Title string
	File  string
	Blurb string
}

var Guides = []Guide{
	{Slug: "switch", Title: "Live switch (all distros)", File: "cloud-init/switch.md", Blurb: "curl|bash installer for already-running servers"},
	{Slug: "windows", Title: "Windows", File: "cloud-init/windows.md", Blurb: "Docs only — not a Windows Update mirror"},
	{Slug: "ubuntu", Title: "Ubuntu", File: "cloud-init/ubuntu.md", Blurb: "cloud-init + live apt (amd64 / ports)"},
	{Slug: "alpine", Title: "Alpine Linux", File: "cloud-init/alpine.md", Blurb: "cloud-init + live apk repositories"},
	{Slug: "centos-stream", Title: "CentOS (Stream)", File: "cloud-init/centos-stream.md", Blurb: "cloud-init + live Stream 9/10 + EPEL"},
	{Slug: "debian", Title: "Debian", File: "cloud-init/debian.md", Blurb: "cloud-init + live main/security sources"},
	{Slug: "almalinux-8", Title: "AlmaLinux 8", File: "cloud-init/almalinux-8.md", Blurb: "cloud-init + live Alma 8 + EPEL"},
	{Slug: "almalinux", Title: "AlmaLinux 9", File: "cloud-init/almalinux.md", Blurb: "cloud-init + live split repos + EPEL"},
	{Slug: "almalinux-10", Title: "AlmaLinux 10", File: "cloud-init/almalinux-10.md", Blurb: "cloud-init + live Alma 10 repos + EPEL"},
	{Slug: "rocky-8", Title: "Rocky Linux 8", File: "cloud-init/rocky-8.md", Blurb: "cloud-init + live rocky*.repo patch"},
	{Slug: "rocky", Title: "Rocky Linux 9", File: "cloud-init/rocky.md", Blurb: "cloud-init + live rocky*.repo patch"},
	{Slug: "rocky-10", Title: "Rocky Linux 10", File: "cloud-init/rocky-10.md", Blurb: "cloud-init + live rocky*.repo patch"},
	{Slug: "arch", Title: "Arch Linux", File: "cloud-init/arch.md", Blurb: "cloud-init + live pacman mirrorlist"},
	{Slug: "cpanel", Title: "cPanel / WHM", File: "cloud-init/cpanel.md", Blurb: "FastUpdate HTTPUPDATE / hosts + --cpanel-hosts"},
}

type Site struct {
	cfg     *config.Config
	store   *store.Store
	metrics *metrics.Metrics
	handler *mirror.Handler
	tmpl    *template.Template
	md      goldmark.Markdown

	starsMu    sync.Mutex
	starsCount *int
	starsAt    time.Time
}

type pageData struct {
	Title       string
	Active      string
	Wide        bool
	GithubURL   string
	GithubRepo  string
	GithubStars any
	Content     template.HTML
	// page-specific
	Paths   []string
	Guides  []Guide
	Guide   *Guide
	BodyHTML template.HTML
	// metrics
	Bucket        string
	Hits          int64
	Misses        int64
	HitPct        int
	Circumference string
	Offset        string
	BytesServed   int64
	Inflight      int64
	InflightPeak  int64
	TmpFree       int64
	S3Used        int64
	S3Objs        int64
	S3FreeLabel   string
	S3QuotaLabel  string
	UsagePct      string
	UsageNote     string
	Details       []detailItem
	HitsFmt       string
	MissesFmt     string
	BytesFmt      string
	InflightFmt   string
	PeakFmt       string
	TmpFmt        string
	S3UsedFmt     string
	S3ObjsFmt     string
	MixNote       string
}

type detailItem struct {
	ID    string
	Label string
	Value string
	Tone  string
}

func New(cfg *config.Config, st *store.Store, m *metrics.Metrics, mh *mirror.Handler) (*Site, error) {
	tmplPath := filepath.Join(cfg.WebRoot, "templates", "*.html")
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"fmtInt":   fmtInt,
		"fmtBytes": fmtBytes,
	}).ParseGlob(tmplPath)
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Table),
		goldmark.WithRendererOptions(gmhtml.WithHardWraps(), gmhtml.WithUnsafe()),
	)
	return &Site{cfg: cfg, store: st, metrics: m, handler: mh, tmpl: tmpl, md: md}, nil
}

func (s *Site) Register(mux *http.ServeMux) {
	// GET registration also serves HEAD (Go 1.22+). Use /{$} for exact home so it
	// does not conflict with the mirror catch-all.
	staticDir := filepath.Join(s.cfg.WebRoot, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /api", s.apiRoot)
	mux.HandleFunc("GET /api/metrics", s.apiMetrics)
	mux.HandleFunc("GET /metrics", s.metricsPage)
	mux.HandleFunc("GET /{$}", s.home)
	mux.HandleFunc("GET /guides", s.guidesIndex)
	mux.HandleFunc("GET /guides/{slug}", s.guideDetail)
	mux.HandleFunc("GET /switch-mirror.sh", s.switchScript)
	mux.Handle("GET /{path...}", s.handler)
}

func (s *Site) githubStars() any {
	s.starsMu.Lock()
	defer s.starsMu.Unlock()
	if s.starsCount != nil && time.Since(s.starsAt) < githubStarsTTL {
		return *s.starsCount
	}
	stale := s.starsCount
	client := &http.Client{Timeout: 4 * time.Second}
	req, _ := http.NewRequest(http.MethodGet, githubAPIURL, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "scalewithus-linux-mirror")
	resp, err := client.Do(req)
	if err != nil {
		slog.Debug("GitHub stars fetch failed", "err", err)
		if stale != nil {
			return *stale
		}
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		if stale != nil {
			return *stale
		}
		return nil
	}
	var payload struct {
		StargazersCount int `json:"stargazers_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		if stale != nil {
			return *stale
		}
		return nil
	}
	s.starsCount = &payload.StargazersCount
	s.starsAt = time.Now()
	return payload.StargazersCount
}

func (s *Site) baseData(title, active string, wide bool) pageData {
	return pageData{
		Title:       title,
		Active:      active,
		Wide:        wide,
		GithubURL:   githubURL,
		GithubRepo:  githubRepo,
		GithubStars: s.githubStars(),
	}
}

func (s *Site) render(w http.ResponseWriter, name string, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		slog.Error("template render", "name", name, "err", err)
		http.Error(w, "template error\n", http.StatusInternalServerError)
	}
}

func (s *Site) snap() map[string]any {
	snap := s.metrics.Snapshot()
	snap["tmp_free_bytes"] = mirror.TmpFreeBytes()
	for k, v := range s.store.Usage.Snapshot(s.cfg.S3QuotaBytes) {
		snap[k] = v
	}
	return snap
}

func (s *Site) healthz(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{
		"status":         "ok",
		"bucket":         s.cfg.S3Bucket,
		"tmp_free_bytes":    mirror.TmpFreeBytes(),
		"inflight":          s.handler.Inflight(),
		"head_cache_entries": s.handler.HeadCacheLen(),
	}
	for k, v := range s.store.Usage.Snapshot(s.cfg.S3QuotaBytes) {
		out[k] = v
	}
	writeJSON(w, out)
}

func (s *Site) apiRoot(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"mirror":        "mirror.scalewithus.com",
		"paths":         upstream.Paths(),
		"cache":         "s3-on-demand",
		"metrics":       "/metrics",
		"guides":        "/guides",
		"switch_script": "/switch-mirror.sh",
	})
}

func (s *Site) apiMetrics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.snap())
}

func (s *Site) home(w http.ResponseWriter, r *http.Request) {
	accept := strings.ToLower(r.Header.Get("Accept"))
	if strings.Contains(accept, "application/json") && !strings.Contains(accept, "text/html") {
		s.apiRoot(w, r)
		return
	}
	d := s.baseData("Home", "home", false)
	d.Paths = upstream.Paths()
	d.Guides = Guides
	s.render(w, "home.html", d)
}

func (s *Site) guidesIndex(w http.ResponseWriter, r *http.Request) {
	d := s.baseData("Guides", "guides", false)
	d.Guides = Guides
	s.render(w, "guides_index.html", d)
}

func (s *Site) guideDetail(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	var g *Guide
	for i := range Guides {
		if Guides[i].Slug == slug {
			g = &Guides[i]
			break
		}
	}
	if g == nil {
		http.Error(w, "Guide not found. See /guides\n", http.StatusNotFound)
		return
	}
	path := filepath.Join(s.cfg.DocsRoot, g.File)
	raw, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, "Guide not found. See /guides\n", http.StatusNotFound)
		return
	}
	d := s.baseData(g.Title, "guides", false)
	d.Guide = g
	d.BodyHTML = template.HTML(mdToHTML(s.md, string(raw)))
	s.render(w, "guide.html", d)
}

func (s *Site) metricsPage(w http.ResponseWriter, r *http.Request) {
	accept := strings.ToLower(r.Header.Get("Accept"))
	snap := s.snap()
	if strings.Contains(accept, "application/json") && !strings.Contains(accept, "text/html") {
		writeJSON(w, snap)
		return
	}
	hits := asInt64(snap["hits_s3"])
	misses := asInt64(snap["misses_stored"])
	total := hits + misses
	hitPct := 0.0
	if total > 0 {
		hitPct = 100.0 * float64(hits) / float64(total)
	}
	circumference := 2 * 3.1415926535 * 42
	offset := circumference * (1 - hitPct/100.0)

	s3Used := asInt64(snap["s3_used_bytes"])
	s3Objs := asInt64(snap["s3_object_count"])
	var s3FreeLabel, s3QuotaLabel, usageNote string
	usagePct := 0.0
	if q, ok := snap["s3_quota_bytes"].(int64); ok && q > 0 {
		usagePct = min(100.0, 100.0*float64(s3Used)/float64(q))
		s3FreeLabel = fmtBytes(asInt64(snap["s3_free_bytes"]))
		s3QuotaLabel = fmtBytes(q)
		usageNote = "Quota " + s3QuotaLabel
	} else if q, ok := snap["s3_quota_bytes"].(float64); ok && q > 0 {
		usagePct = min(100.0, 100.0*float64(s3Used)/q)
		s3FreeLabel = fmtBytes(asInt64(snap["s3_free_bytes"]))
		s3QuotaLabel = fmtBytes(int64(q))
		usageNote = "Quota " + s3QuotaLabel
	} else {
		s3FreeLabel = "—"
		s3QuotaLabel = "—"
		usageNote = "Set S3_QUOTA_BYTES to show free space"
	}
	if errStr, ok := snap["s3_usage_error"].(string); ok && errStr != "" {
		usageNote = "Usage refresh error: " + html.EscapeString(errStr)
	}

	d := s.baseData("Metrics", "metrics", true)
	d.Bucket = s.cfg.S3Bucket
	d.Hits = hits
	d.Misses = misses
	d.HitPct = int(hitPct + 0.5)
	d.Circumference = fmt.Sprintf("%.2f", circumference)
	d.Offset = fmt.Sprintf("%.2f", offset)
	d.BytesServed = asInt64(snap["bytes_served"])
	d.Inflight = asInt64(snap["inflight"])
	d.InflightPeak = asInt64(snap["inflight_peak"])
	d.TmpFree = asInt64(snap["tmp_free_bytes"])
	d.S3Used = s3Used
	d.S3Objs = s3Objs
	d.S3FreeLabel = s3FreeLabel
	d.S3QuotaLabel = s3QuotaLabel
	d.UsagePct = fmt.Sprintf("%.1f", usagePct)
	d.UsageNote = usageNote
	d.HitsFmt = fmtInt(hits)
	d.MissesFmt = fmtInt(misses)
	d.BytesFmt = fmtBytes(d.BytesServed)
	d.InflightFmt = fmtInt(d.Inflight)
	d.PeakFmt = fmtInt(d.InflightPeak)
	d.TmpFmt = fmtBytes(d.TmpFree)
	d.S3UsedFmt = fmtBytes(s3Used)
	d.S3ObjsFmt = fmtInt(s3Objs)
	d.MixNote = fmt.Sprintf("%s hits · %s misses stored", d.HitsFmt, d.MissesFmt)
	d.Details = []detailItem{
		{"m-reval", "Revalidated 304", fmtInt(asInt64(snap["revalidated_304"])), ""},
		{"m-range", "Range / 206", fmtInt(asInt64(snap["range_hits"])), ""},
		{"m-neg", "Negative cache hits", fmtInt(asInt64(snap["negative_hits"])), ""},
		{"m-nf", "Not found", fmtInt(asInt64(snap["not_found"])), ""},
		{"m-err", "Upstream errors", fmtInt(asInt64(snap["upstream_errors"])), "danger"},
		{"m-storefail", "Store failed", fmtInt(asInt64(snap["misses_store_failed"])), "warn"},
		{"m-conflict", "Package conflicts", fmtInt(asInt64(snap["package_conflicts"])), "warn"},
		{"m-neg-entries", "Neg. cache entries", fmtInt(asInt64(snap["negative_cache_entries"])), ""},
		{"m-validated", "Validated meta", fmtInt(asInt64(snap["validated_entries"])), ""},
	}
	s.render(w, "metrics.html", d)
}

func (s *Site) switchScript(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(s.cfg.ScriptsRoot, "switch-mirror.sh")
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, "switch-mirror.sh not found\n", http.StatusNotFound)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		http.Error(w, "switch-mirror.sh not found\n", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/x-shellscript")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("Content-Disposition", `inline; filename="switch-mirror.sh"`)
	http.ServeContent(w, r, "switch-mirror.sh", st.ModTime(), f)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func fmtInt(n int64) string {
	s := strconv.FormatInt(n, 10)
	if n < 0 {
		return s
	}
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return b.String()
}

func fmtBytes(n int64) string {
	val := float64(n)
	units := []string{"B", "KB", "MB", "GB", "TB"}
	for i, u := range units {
		if val < 1024 || u == "TB" {
			if u == "B" {
				return fmt.Sprintf("%d %s", int64(val), u)
			}
			return fmt.Sprintf("%.1f %s", val, u)
		}
		if i < len(units)-1 {
			val /= 1024
		}
	}
	return fmt.Sprintf("%.1f TB", val)
}

func asInt64(v any) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int:
		return int64(t)
	case float64:
		return int64(t)
	default:
		return 0
	}
}

var mdLinkRE = regexp.MustCompile(`\]\(([\w.-]+\.md)(#[\w.-]+)?\)`)
var preRE = regexp.MustCompile(`(?s)<pre(?:\s[^>]*)?>.*?</pre>`)

func mdToHTML(md goldmark.Markdown, text string) string {
	text = mdLinkRE.ReplaceAllStringFunc(text, func(m string) string {
		sub := mdLinkRE.FindStringSubmatch(m)
		if sub == nil {
			return m
		}
		slug := strings.TrimSuffix(sub[1], ".md")
		frag := ""
		if len(sub) > 2 {
			frag = sub[2]
		}
		return "](/guides/" + slug + frag + ")"
	})
	var buf bytes.Buffer
	if err := md.Convert([]byte(text), &buf); err != nil {
		return html.EscapeString(text)
	}
	return wrapCodeBlocks(buf.String())
}

func wrapCodeBlocks(rendered string) string {
	return preRE.ReplaceAllStringFunc(rendered, func(block string) string {
		lang := ""
		if m := regexp.MustCompile(`class="([^"]*)"`).FindStringSubmatch(block); m != nil {
			for _, part := range strings.Fields(m[1]) {
				if strings.HasPrefix(part, "language-") {
					lang = strings.TrimPrefix(part, "language-")
					break
				}
			}
		}
		langHTML := ""
		if lang != "" {
			langHTML = `<span class="lang-tag">` + html.EscapeString(lang) + `</span>`
		}
		return `<div class="code-block">` + langHTML +
			`<button type="button" class="copy-btn" aria-label="Copy to clipboard">Copy</button>` +
			block + `</div>`
	})
}

// AccessLog middleware
func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		rw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(rw, r)
		client := r.RemoteAddr
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			client = strings.TrimSpace(strings.Split(xff, ",")[0])
		}
		xcache := rw.Header().Get("X-Cache")
		if xcache == "" {
			xcache = "-"
		}
		slog.Info(fmt.Sprintf(`%s "%s %s" %d %s`, client, r.Method, r.URL.Path, rw.status, xcache))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	return w.ResponseWriter.Write(b)
}
