package metrics

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

var persistKeys = []string{
	"hits_s3",
	"hits_disk",
	"misses_stored",
	"misses_store_failed",
	"upstream_errors",
	"not_found",
	"negative_hits",
	"revalidated_304",
	"range_hits",
	"package_conflicts",
	"bytes_served",
	"inflight_peak",
	"disk_evictions",
}

type Metrics struct {
	HitsS3            atomic.Int64
	HitsDisk          atomic.Int64
	MissesStored      atomic.Int64
	MissesStoreFailed atomic.Int64
	UpstreamErrors    atomic.Int64
	NotFound          atomic.Int64
	NegativeHits      atomic.Int64
	Revalidated304    atomic.Int64
	RangeHits         atomic.Int64
	PackageConflicts  atomic.Int64
	BytesServed       atomic.Int64
	InflightPeak      atomic.Int64
	DiskEvictions     atomic.Int64

	dirty atomic.Bool

	inflight         atomic.Int64
	negEntries       atomic.Int64
	validatedEntries atomic.Int64
}

func (m *Metrics) Incr(name string, n int64) {
	if n == 0 {
		n = 1
	}
	switch name {
	case "hits_s3":
		m.HitsS3.Add(n)
	case "hits_disk":
		m.HitsDisk.Add(n)
	case "misses_stored":
		m.MissesStored.Add(n)
	case "misses_store_failed":
		m.MissesStoreFailed.Add(n)
	case "upstream_errors":
		m.UpstreamErrors.Add(n)
	case "not_found":
		m.NotFound.Add(n)
	case "negative_hits":
		m.NegativeHits.Add(n)
	case "revalidated_304":
		m.Revalidated304.Add(n)
	case "range_hits":
		m.RangeHits.Add(n)
	case "package_conflicts":
		m.PackageConflicts.Add(n)
	case "bytes_served":
		m.BytesServed.Add(n)
	case "disk_evictions":
		m.DiskEvictions.Add(n)
	}
	cur := m.inflight.Load()
	for {
		peak := m.InflightPeak.Load()
		if cur <= peak {
			break
		}
		if m.InflightPeak.CompareAndSwap(peak, cur) {
			break
		}
	}
	m.dirty.Store(true)
}

func (m *Metrics) SetInflight(n int64) {
	m.inflight.Store(n)
	for {
		peak := m.InflightPeak.Load()
		if n <= peak {
			break
		}
		if m.InflightPeak.CompareAndSwap(peak, n) {
			m.dirty.Store(true)
			break
		}
	}
}

func (m *Metrics) AddInflight(delta int64) int64 {
	n := m.inflight.Add(delta)
	m.SetInflight(n)
	return n
}

func (m *Metrics) SetNegEntries(n int64)      { m.negEntries.Store(n) }
func (m *Metrics) SetValidatedEntries(n int64) { m.validatedEntries.Store(n) }

func (m *Metrics) Snapshot() map[string]any {
	return map[string]any{
		"hits_s3":                m.HitsS3.Load(),
		"hits_disk":              m.HitsDisk.Load(),
		"misses_stored":          m.MissesStored.Load(),
		"misses_store_failed":    m.MissesStoreFailed.Load(),
		"upstream_errors":        m.UpstreamErrors.Load(),
		"not_found":              m.NotFound.Load(),
		"negative_hits":          m.NegativeHits.Load(),
		"revalidated_304":        m.Revalidated304.Load(),
		"range_hits":             m.RangeHits.Load(),
		"package_conflicts":      m.PackageConflicts.Load(),
		"bytes_served":           m.BytesServed.Load(),
		"inflight":               m.inflight.Load(),
		"inflight_peak":          m.InflightPeak.Load(),
		"negative_cache_entries": m.negEntries.Load(),
		"validated_entries":      m.validatedEntries.Load(),
		"disk_evictions":         m.DiskEvictions.Load(),
	}
}

func (m *Metrics) persistable() map[string]any {
	return map[string]any{
		"hits_s3":             m.HitsS3.Load(),
		"hits_disk":           m.HitsDisk.Load(),
		"misses_stored":       m.MissesStored.Load(),
		"misses_store_failed": m.MissesStoreFailed.Load(),
		"upstream_errors":     m.UpstreamErrors.Load(),
		"not_found":           m.NotFound.Load(),
		"negative_hits":       m.NegativeHits.Load(),
		"revalidated_304":     m.Revalidated304.Load(),
		"range_hits":          m.RangeHits.Load(),
		"package_conflicts":   m.PackageConflicts.Load(),
		"bytes_served":        m.BytesServed.Load(),
		"inflight_peak":       m.InflightPeak.Load(),
		"disk_evictions":      m.DiskEvictions.Load(),
	}
}

func (m *Metrics) LoadFromDisk(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		slog.Warn("Failed to load metrics", "path", path, "err", err)
		return false
	}
	for _, key := range persistKeys {
		v, ok := raw[key]
		if !ok {
			continue
		}
		var n int64
		switch t := v.(type) {
		case float64:
			n = int64(t)
		default:
			continue
		}
		switch key {
		case "hits_s3":
			m.HitsS3.Store(n)
		case "hits_disk":
			m.HitsDisk.Store(n)
		case "misses_stored":
			m.MissesStored.Store(n)
		case "misses_store_failed":
			m.MissesStoreFailed.Store(n)
		case "upstream_errors":
			m.UpstreamErrors.Store(n)
		case "not_found":
			m.NotFound.Store(n)
		case "negative_hits":
			m.NegativeHits.Store(n)
		case "revalidated_304":
			m.Revalidated304.Store(n)
		case "range_hits":
			m.RangeHits.Store(n)
		case "package_conflicts":
			m.PackageConflicts.Store(n)
		case "bytes_served":
			m.BytesServed.Store(n)
		case "inflight_peak":
			m.InflightPeak.Store(n)
		case "disk_evictions":
			m.DiskEvictions.Store(n)
		}
	}
	m.dirty.Store(false)
	return true
}

func (m *Metrics) SaveToDisk(path string) error {
	payload := m.persistable()
	payload["saved_at"] = float64(time.Now().Unix()) + float64(time.Now().Nanosecond())/1e9
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	m.dirty.Store(false)
	return nil
}

func (m *Metrics) Dirty() bool { return m.dirty.Load() }

func (m *Metrics) FlushLoop(path string, every time.Duration, stop <-chan struct{}) {
	if every < time.Second {
		every = time.Second
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			if !m.Dirty() {
				continue
			}
			if err := m.SaveToDisk(path); err != nil {
				slog.Warn("Metrics flush failed", "err", err)
			}
		}
	}
}
