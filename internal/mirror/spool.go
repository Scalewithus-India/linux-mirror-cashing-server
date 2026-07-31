package mirror

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"syscall"

	"github.com/Scalewithus-India/linux-mirror-cashing-server/internal/store"
	"github.com/Scalewithus-India/linux-mirror-cashing-server/internal/upstream"
)

func tmpFreeBytes() int64 {
	var st syscall.Statfs_t
	dir := os.TempDir()
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0
	}
	return int64(st.Bavail) * int64(st.Bsize)
}

func (h *Handler) ensureSpoolCapacity(expected int64) error {
	free := tmpFreeBytes()
	need := h.cfg.MinTmpFreeBytes
	if expected > 0 {
		need = max(need, expected+h.cfg.MinTmpFreeBytes)
	}
	if free < need {
		return fmt.Errorf("insufficient tmp space: free=%d need>=%d tmp=%s", free, need, os.TempDir())
	}
	return nil
}

type spoolResult struct {
	path   string
	size   int64
	ctype  string
	xcache string
}

func (h *Handler) fetchAndUpload(ctx context.Context, key, upstreamURL string, existing *store.ObjectHead) (*spoolResult, int, []byte, string, error) {
	if err := h.ensureSpoolCapacity(0); err != nil {
		return nil, http.StatusServiceUnavailable, []byte("Insufficient temporary storage\n"), "SPOOL-NO-SPACE", nil
	}

	select {
	case h.spoolSem <- struct{}{}:
		defer func() { <-h.spoolSem }()
	case <-ctx.Done():
		return nil, http.StatusServiceUnavailable, []byte("Cancelled\n"), "SPOOL-CANCELLED", ctx.Err()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamURL, nil)
	if err != nil {
		return nil, http.StatusBadGateway, []byte("Bad upstream URL\n"), "UPSTREAM-ERROR", err
	}
	req.Header.Set("User-Agent", "scalewithus-linux-mirror/1.0")
	resp, err := h.httpClient.Do(req)
	if err != nil {
		h.metrics.Incr("upstream_errors", 1)
		return nil, http.StatusBadGateway, []byte("Upstream fetch failed\n"), "UPSTREAM-ERROR", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		h.rememberNegative(key)
		h.metrics.Incr("not_found", 1)
		return nil, http.StatusNotFound, []byte("Not Found"), "MISS", nil
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		h.metrics.Incr("upstream_errors", 1)
		return nil, http.StatusBadGateway, []byte("Upstream redirect refused\n"), "UPSTREAM-REDIRECT", nil
	}
	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, h.cfg.MaxUpstreamErrorBytes))
		h.metrics.Incr("upstream_errors", 1)
		return nil, resp.StatusCode, data, "UPSTREAM-ERROR", nil
	}

	ctype := upstream.ResponseContentType(key, resp.Header.Get("Content-Type"))
	declared := resp.Header.Get("Content-Length")
	var expected int64
	if declared != "" {
		if n, e := strconv.ParseInt(declared, 10, 64); e == nil {
			expected = n
		}
	}
	if expected > 0 && expected > h.cfg.MaxSpoolBytes {
		h.metrics.Incr("upstream_errors", 1)
		return nil, http.StatusBadGateway, []byte("Object exceeds MAX_SPOOL_BYTES\n"), "SPOOL-TOO-LARGE", nil
	}
	if expected > 0 {
		if err := h.ensureSpoolCapacity(expected); err != nil {
			return nil, http.StatusServiceUnavailable, []byte("Insufficient temporary storage\n"), "SPOOL-NO-SPACE", nil
		}
	}

	tmp, err := os.CreateTemp("", "mirror-*.bin")
	if err != nil {
		return nil, http.StatusInternalServerError, []byte("tmp create failed\n"), "SPOOL-ERROR", err
	}
	tmpPath := tmp.Name()
	var size int64
	bp := copyBufPool.Get().(*[]byte)
	buf := *bp
	cleanup := true
	defer func() {
		copyBufPool.Put(bp)
		_ = tmp.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			size += int64(n)
			if size > h.cfg.MaxSpoolBytes {
				h.metrics.Incr("upstream_errors", 1)
				return nil, http.StatusBadGateway, []byte("Object exceeds MAX_SPOOL_BYTES\n"), "SPOOL-TOO-LARGE", nil
			}
			if size == int64(n) && tmpFreeBytes() < h.cfg.MinTmpFreeBytes {
				return nil, http.StatusServiceUnavailable, []byte("Insufficient temporary storage\n"), "SPOOL-NO-SPACE", nil
			}
			if _, werr := tmp.Write(buf[:n]); werr != nil {
				return nil, http.StatusInternalServerError, []byte("spool write failed\n"), "SPOOL-ERROR", werr
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			h.metrics.Incr("upstream_errors", 1)
			return nil, http.StatusBadGateway, []byte("Upstream read failed\n"), "UPSTREAM-ERROR", readErr
		}
	}
	if err := tmp.Sync(); err != nil {
		return nil, http.StatusInternalServerError, []byte("spool sync failed\n"), "SPOOL-ERROR", err
	}
	if err := tmp.Close(); err != nil {
		return nil, http.StatusInternalServerError, []byte("spool close failed\n"), "SPOOL-ERROR", err
	}

	xcache, err := h.uploadSpool(ctx, key, tmpPath, ctype, size, existing)
	if err != nil {
		return nil, http.StatusBadGateway, []byte("S3 upload failed\n"), "MISS-STORE-FAILED", err
	}
	cleanup = false
	return &spoolResult{path: tmpPath, size: size, ctype: ctype, xcache: xcache}, 0, nil, "", nil
}

func (h *Handler) uploadSpool(ctx context.Context, key, tmpPath, ctype string, size int64, existing *store.ObjectHead) (string, error) {
	if existing != nil && !upstream.IsMetadataKey(key) {
		if existing.ContentLength != size {
			h.metrics.Incr("package_conflicts", 1)
			h.heads.Delete(key)
			if h.disk != nil {
				h.disk.Delete(key)
			}
			slog.Warn("PACKAGE CONFLICT", "bucket", h.store.Bucket, "key", key,
				"existing", existing.ContentLength, "new", size)
			return "MISS-STORE-SKIPPED-CONFLICT", nil
		}
	}
	ctype = upstream.ResponseContentType(key, ctype)
	if err := h.store.PutFile(ctx, key, ctype, tmpPath, size); err != nil {
		h.metrics.Incr("misses_store_failed", 1)
		slog.Error("S3 upload failed", "key", key, "err", err)
		return "MISS-STORE-FAILED", err
	}
	slog.Info("CACHED", "object", store.FormatBucketURL(h.store.Bucket, key), "bytes", size)
	h.metrics.Incr("misses_stored", 1)
	var replaced *int64
	if existing != nil {
		sz := existing.ContentLength
		replaced = &sz
	}
	h.store.Usage.NoteStore(size, replaced)
	h.clearNegative(key)
	if upstream.IsMetadataKey(key) {
		h.markValidated(key)
	}
	h.populateDiskFile(key, ctype, tmpPath, size)
	return "MISS-STORED", nil
}

func (h *Handler) serveFromSpool(w http.ResponseWriter, r *http.Request, key string, sp *spoolResult, rangeHeader string) {
	defer os.Remove(sp.path)

	start, end, err := parseBytesRange(rangeHeader, sp.size)
	if errors.Is(err, errUnsatisfiableRange) {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", sp.size))
		w.Header().Set("X-Cache", sp.xcache)
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	if err != nil {
		start, end = -1, -1
	}

	w.Header().Set("Cache-Control", h.cacheControl(key))
	w.Header().Set("X-Cache", sp.xcache)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", sp.ctype)

	f, err := os.Open(sp.path)
	if err != nil {
		http.Error(w, "spool open failed\n", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	if start < 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(sp.size, 10))
		h.metrics.Incr("bytes_served", sp.size)
		_, _ = copyBuf(w, f)
		return
	}
	length := end - start + 1
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, sp.size))
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	h.metrics.Incr("range_hits", 1)
	h.metrics.Incr("bytes_served", length)
	w.WriteHeader(http.StatusPartialContent)
	section := io.NewSectionReader(f, start, length)
	_, _ = copyBuf(w, section)
}
