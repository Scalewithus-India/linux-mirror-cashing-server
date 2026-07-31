package mirror

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Scalewithus-India/linux-mirror-cashing-server/internal/config"
	"github.com/Scalewithus-India/linux-mirror-cashing-server/internal/metrics"
	"github.com/Scalewithus-India/linux-mirror-cashing-server/internal/store"
	"github.com/Scalewithus-India/linux-mirror-cashing-server/internal/upstream"
)

type flight struct {
	done chan struct{}
}

type Handler struct {
	cfg        *config.Config
	store      *store.Store
	metrics    *metrics.Metrics
	httpClient *http.Client
	spoolSem   chan struct{}
	heads      *headCache

	metaCC string
	pkgCC  string
	negCC  string

	flightMu sync.Mutex
	inflight map[string]*flight

	negMu    sync.Mutex
	negative map[string]time.Time

	valMu     sync.Mutex
	validated map[string]time.Time
}

func New(cfg *config.Config, st *store.Store, m *metrics.Metrics) *Handler {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          256,
		MaxIdleConnsPerHost:   32,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: cfg.UpstreamTimeout,
	}
	return &Handler{
		cfg:   cfg,
		store: st,
		metrics: m,
		httpClient: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Transport: transport,
		},
		spoolSem: make(chan struct{}, cfg.MaxConcurrentSpools),
		heads:    newHeadCache(cfg.HeadCacheMax),
		metaCC:   fmt.Sprintf("public, max-age=%d", cfg.MetadataCacheSeconds),
		pkgCC:    fmt.Sprintf("public, max-age=%d, immutable", cfg.PackageCacheSeconds),
		negCC:    fmt.Sprintf("public, max-age=%d", cfg.NegativeCacheSeconds),
		inflight: make(map[string]*flight),
		negative: make(map[string]time.Time),
		validated: make(map[string]time.Time),
	}
}

func (h *Handler) Inflight() int {
	h.flightMu.Lock()
	defer h.flightMu.Unlock()
	return len(h.inflight)
}

func (h *Handler) HeadCacheLen() int { return h.heads.Len() }

func (h *Handler) cacheControl(key string) string {
	if upstream.IsMetadataKey(key) {
		return h.metaCC
	}
	return h.pkgCC
}

func (h *Handler) rememberHead(key string, head *store.ObjectHead) {
	if head == nil {
		return
	}
	if upstream.IsMetadataKey(key) {
		h.heads.Put(key, head, time.Duration(h.cfg.MetadataCacheSeconds)*time.Second)
		return
	}
	h.heads.Put(key, head, 0)
}

func (h *Handler) rememberNegative(key string) {
	h.negMu.Lock()
	defer h.negMu.Unlock()
	h.negative[key] = time.Now().Add(time.Duration(h.cfg.NegativeCacheSeconds) * time.Second)
	h.metrics.SetNegEntries(int64(len(h.negative)))
	if len(h.negative) > 10000 {
		now := time.Now()
		for k, e := range h.negative {
			if !e.After(now) {
				delete(h.negative, k)
			}
		}
		h.metrics.SetNegEntries(int64(len(h.negative)))
	}
}

func (h *Handler) negativeCached(key string) bool {
	h.negMu.Lock()
	defer h.negMu.Unlock()
	exp, ok := h.negative[key]
	if !ok {
		return false
	}
	if time.Now().Before(exp) {
		return true
	}
	delete(h.negative, key)
	h.metrics.SetNegEntries(int64(len(h.negative)))
	return false
}

func (h *Handler) clearNegative(key string) {
	h.negMu.Lock()
	defer h.negMu.Unlock()
	delete(h.negative, key)
	h.metrics.SetNegEntries(int64(len(h.negative)))
}

func (h *Handler) markValidated(key string) {
	h.valMu.Lock()
	defer h.valMu.Unlock()
	h.validated[key] = time.Now()
	h.metrics.SetValidatedEntries(int64(len(h.validated)))
}

func (h *Handler) s3ObjectIsFresh(head *store.ObjectHead, key string) bool {
	if !upstream.IsMetadataKey(key) {
		return true
	}
	h.valMu.Lock()
	validated, ok := h.validated[key]
	h.valMu.Unlock()
	ttl := time.Duration(h.cfg.MetadataCacheSeconds) * time.Second
	if ok && time.Since(validated) <= ttl {
		return true
	}
	if head.LastModified.IsZero() {
		return false
	}
	return time.Since(head.LastModified) <= ttl
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed\n", http.StatusMethodNotAllowed)
		return
	}
	path := upstream.NormalizePath(r.URL.Path)
	if path == "" {
		http.Error(w, "Invalid path\n", http.StatusBadRequest)
		return
	}
	resolved := upstream.Resolve(path)
	if resolved == nil {
		resolved = upstream.Resolve(path + "/")
	}
	if resolved == nil {
		http.Error(w, "Unknown mirror prefix. See GET / for paths.\n", http.StatusNotFound)
		return
	}
	key := resolved.Key
	upstreamURL := resolved.UpstreamURL
	rangeHeader := r.Header.Get("Range")

	if strings.HasSuffix(key, "/") {
		w.Header().Set("X-Cache", "DIR-DISABLED")
		w.Header().Set("Cache-Control", h.negCC)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("Directory listings are not proxied\n"))
		return
	}

	if h.negativeCached(key) {
		h.metrics.Incr("negative_hits", 1)
		w.Header().Set("X-Cache", "NEGATIVE")
		w.Header().Set("Cache-Control", h.negCC)
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	ctx := r.Context()

	// Packages are immutable: skip HeadObject when possible (one RTT instead of two).
	if !upstream.IsMetadataKey(key) {
		if head := h.heads.Get(key); head != nil {
			if r.Method == http.MethodHead {
				h.headResponseS3(w, key, head, rangeHeader)
				return
			}
			h.streamS3(w, r, key, head, rangeHeader)
			return
		}
		if r.Method == http.MethodHead {
			head, err := h.store.Head(ctx, key)
			if err == nil {
				h.rememberHead(key, head)
				h.headResponseS3(w, key, head, rangeHeader)
				return
			}
			if !store.IsNotFound(err) {
				slog.Debug("S3 head", "key", key, "err", err)
			}
			h.headMiss(w, ctx, key, upstreamURL)
			return
		}
		if h.tryPackageGet(w, r, key, rangeHeader) {
			return
		}
		h.fetchUpstreamAndCache(w, r, key, upstreamURL, nil, rangeHeader)
		return
	}

	// Metadata: Head (or cache) → freshness / revalidate → Get
	head := h.heads.Get(key)
	var err error
	if head == nil {
		head, err = h.store.Head(ctx, key)
		if err == nil {
			h.rememberHead(key, head)
		}
	}
	if err == nil && head != nil {
		if h.s3ObjectIsFresh(head, key) {
			if r.Method == http.MethodHead {
				h.headResponseS3(w, key, head, rangeHeader)
				return
			}
			h.streamS3(w, r, key, head, rangeHeader)
			return
		}
		if h.revalidateMetadata(ctx, key, upstreamURL, head) {
			if r.Method == http.MethodHead {
				h.headResponseS3(w, key, head, rangeHeader)
				return
			}
			h.streamS3(w, r, key, head, rangeHeader)
			return
		}
		slog.Info("S3 stale metadata, re-fetching", "key", key)
		h.heads.Delete(key)
	} else if err != nil && !store.IsNotFound(err) {
		slog.Debug("S3 head", "key", key, "err", err)
		head = nil
	} else {
		head = nil
	}

	if r.Method == http.MethodHead {
		h.headMiss(w, ctx, key, upstreamURL)
		return
	}
	h.fetchUpstreamAndCache(w, r, key, upstreamURL, head, rangeHeader)
}

// tryPackageGet streams from S3 without a prior Head. Returns false on miss.
func (h *Handler) tryPackageGet(w http.ResponseWriter, r *http.Request, key, rangeHeader string) bool {
	ctx := r.Context()
	byteRange := ""
	if rangeHeader != "" && !strings.Contains(rangeHeader, ",") {
		byteRange = strings.TrimSpace(rangeHeader)
	}
	res, err := h.store.Get(ctx, key, byteRange)
	if err != nil {
		if store.IsNotFound(err) {
			return false
		}
		if store.IsInvalidRange(err) {
			w.Header().Set("Content-Range", "bytes */*")
			w.Header().Set("X-Cache", "HIT-S3")
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return true
		}
		slog.Debug("S3 get", "key", key, "err", err)
		return false
	}
	defer res.Body.Close()

	head := &store.ObjectHead{
		ContentLength: res.ObjectSize,
		ContentType:   res.ContentType,
		LastModified:  res.LastModified,
	}
	if head.ContentLength == 0 && res.StatusCode == http.StatusOK {
		head.ContentLength = res.ContentLength
	}
	h.rememberHead(key, head)

	ctype := upstream.ResponseContentType(key, res.ContentType)
	w.Header().Set("Cache-Control", h.pkgCC)
	w.Header().Set("X-Cache", "HIT-S3")
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", ctype)
	if res.ContentRange != "" {
		w.Header().Set("Content-Range", res.ContentRange)
		h.metrics.Incr("range_hits", 1)
	}
	if res.ContentLength > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(res.ContentLength, 10))
	}
	h.metrics.Incr("hits_s3", 1)
	h.metrics.Incr("bytes_served", res.ContentLength)
	if res.StatusCode == http.StatusPartialContent {
		w.WriteHeader(http.StatusPartialContent)
	}
	_, _ = copyBuf(w, res.Body)
	return true
}

func (h *Handler) headMiss(w http.ResponseWriter, ctx context.Context, key, upstreamURL string) {
	h.flightMu.Lock()
	if f, ok := h.inflight[key]; ok {
		h.flightMu.Unlock()
		<-f.done
		if head, err := h.store.Head(ctx, key); err == nil {
			h.rememberHead(key, head)
			h.headResponseS3(w, key, head, "")
			return
		}
	} else {
		h.flightMu.Unlock()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, upstreamURL, nil)
	if err != nil {
		http.Error(w, "bad upstream\n", http.StatusBadGateway)
		return
	}
	resp, err := h.httpClient.Do(req)
	if err != nil {
		h.metrics.Incr("upstream_errors", 1)
		http.Error(w, "upstream failed\n", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		h.rememberNegative(key)
		h.metrics.Incr("not_found", 1)
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		h.metrics.Incr("upstream_errors", 1)
		w.Header().Set("X-Cache", "UPSTREAM-REDIRECT")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("Upstream redirect refused\n"))
		return
	}
	w.Header().Set("Content-Type", upstream.ResponseContentType(key, resp.Header.Get("Content-Type")))
	w.Header().Set("X-Cache", "MISS")
	w.Header().Set("Cache-Control", h.cacheControl(key))
	w.Header().Set("Accept-Ranges", "bytes")
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		w.Header().Set("Content-Length", cl)
	}
	w.WriteHeader(resp.StatusCode)
}

func (h *Handler) revalidateMetadata(ctx context.Context, key, upstreamURL string, head *store.ObjectHead) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, upstreamURL, nil)
	if err != nil {
		h.markValidated(key)
		return true
	}
	if !head.LastModified.IsZero() {
		req.Header.Set("If-Modified-Since", head.LastModified.UTC().Format(http.TimeFormat))
	}
	resp, err := h.httpClient.Do(req)
	if err != nil {
		slog.Warn("revalidate HEAD failed — keeping S3 copy", "key", key, "err", err)
		h.markValidated(key)
		return true
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotModified:
		h.markValidated(key)
		h.metrics.Incr("revalidated_304", 1)
		slog.Info("REVALIDATED 304", "key", key)
		return true
	case resp.StatusCode == http.StatusNotFound:
		h.rememberNegative(key)
		h.heads.Delete(key)
		return false
	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		slog.Warn("revalidate redirect — keeping S3 copy", "status", resp.StatusCode, "key", key)
		h.markValidated(key)
		return true
	case resp.StatusCode >= 400:
		slog.Warn("revalidate status — keeping S3 copy", "status", resp.StatusCode, "key", key)
		h.markValidated(key)
		return true
	}

	remoteLen := resp.Header.Get("Content-Length")
	if remoteLen != "" {
		n, e := strconv.ParseInt(remoteLen, 10, 64)
		if e == nil && n == head.ContentLength {
			remoteLM := resp.Header.Get("Last-Modified")
			if remoteLM != "" && !head.LastModified.IsZero() {
				if t, e := http.ParseTime(remoteLM); e == nil {
					if !t.After(head.LastModified) {
						h.markValidated(key)
						h.metrics.Incr("revalidated_304", 1)
						return true
					}
				}
			}
		}
	}
	return false
}

func (h *Handler) streamS3(w http.ResponseWriter, r *http.Request, key string, head *store.ObjectHead, rangeHeader string) {
	size := head.ContentLength
	ctype := upstream.ResponseContentType(key, head.ContentType)
	start, end, err := parseBytesRange(rangeHeader, size)
	if errors.Is(err, errUnsatisfiableRange) || (size == 0 && rangeHeader != "") {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", size))
		w.Header().Set("X-Cache", "HIT-S3")
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	if err != nil {
		start, end = -1, -1
	}

	w.Header().Set("Cache-Control", h.cacheControl(key))
	w.Header().Set("X-Cache", "HIT-S3")
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", ctype)

	ctx := r.Context()
	byteRange := ""
	served := size
	if start >= 0 {
		byteRange = fmt.Sprintf("bytes=%d-%d", start, end)
		served = end - start + 1
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
		w.Header().Set("Content-Length", strconv.FormatInt(served, 10))
		h.metrics.Incr("range_hits", 1)
	} else if size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	}
	h.metrics.Incr("hits_s3", 1)
	h.metrics.Incr("bytes_served", served)

	res, err := h.store.Get(ctx, key, byteRange)
	if err != nil {
		if store.IsNotFound(err) {
			h.heads.Delete(key)
		}
		http.Error(w, "S3 get failed\n", http.StatusBadGateway)
		return
	}
	defer res.Body.Close()
	if start >= 0 {
		w.WriteHeader(http.StatusPartialContent)
	}
	_, _ = copyBuf(w, res.Body)
}

func (h *Handler) headResponseS3(w http.ResponseWriter, key string, head *store.ObjectHead, rangeHeader string) {
	size := head.ContentLength
	ctype := upstream.ResponseContentType(key, head.ContentType)
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Cache-Control", h.cacheControl(key))
	w.Header().Set("X-Cache", "HIT-S3")
	w.Header().Set("Accept-Ranges", "bytes")
	start, end, err := parseBytesRange(rangeHeader, size)
	if errors.Is(err, errUnsatisfiableRange) {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", size))
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	if err != nil {
		start, end = -1, -1
	}
	if start < 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
		h.metrics.Incr("hits_s3", 1)
		w.WriteHeader(http.StatusOK)
		return
	}
	length := end - start + 1
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	h.metrics.Incr("hits_s3", 1)
	h.metrics.Incr("range_hits", 1)
	w.WriteHeader(http.StatusPartialContent)
}

func (h *Handler) fetchUpstreamAndCache(w http.ResponseWriter, r *http.Request, key, upstreamURL string, existing *store.ObjectHead, rangeHeader string) {
	h.flightMu.Lock()
	if f, ok := h.inflight[key]; ok {
		h.flightMu.Unlock()
		<-f.done
		if head := h.heads.Get(key); head != nil {
			h.streamS3(w, r, key, head, rangeHeader)
			return
		}
		head, err := h.store.Head(r.Context(), key)
		if err == nil {
			h.rememberHead(key, head)
			h.streamS3(w, r, key, head, rangeHeader)
			return
		}
		if h.negativeCached(key) {
			h.metrics.Incr("negative_hits", 1)
			w.Header().Set("X-Cache", "NEGATIVE")
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		w.Header().Set("X-Cache", "COALESCE-FAIL")
		http.Error(w, "Coalesced fetch failed\n", http.StatusBadGateway)
		return
	}
	f := &flight{done: make(chan struct{})}
	h.inflight[key] = f
	n := len(h.inflight)
	h.flightMu.Unlock()
	h.metrics.SetInflight(int64(n))

	defer func() {
		h.flightMu.Lock()
		delete(h.inflight, key)
		n := len(h.inflight)
		h.flightMu.Unlock()
		h.metrics.SetInflight(int64(n))
		close(f.done)
	}()

	sp, status, body, xcache, _ := h.fetchAndUpload(r.Context(), key, upstreamURL, existing)
	if status != 0 {
		if xcache != "" {
			w.Header().Set("X-Cache", xcache)
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(status)
		_, _ = w.Write(body)
		return
	}
	if sp == nil {
		w.Header().Set("X-Cache", "COALESCE-FAIL")
		http.Error(w, "Coalesced fetch failed\n", http.StatusBadGateway)
		return
	}
	h.rememberHead(key, &store.ObjectHead{
		ContentLength: sp.size,
		ContentType:   sp.ctype,
		LastModified:  time.Now().UTC(),
	})
	h.serveFromSpool(w, r, key, sp, rangeHeader)
}

// TmpFreeBytes reports free space under the process temp directory.
func TmpFreeBytes() int64 { return tmpFreeBytes() }
