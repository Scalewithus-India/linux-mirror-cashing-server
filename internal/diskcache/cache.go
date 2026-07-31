package diskcache

import (
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// Cache is a local on-disk object cache keyed like S3 (relative paths).
type Cache struct {
	root    string
	reserve int64
	// fixedCap: nil = auto from free disk; non-nil 0 = disabled; >0 = fixed budget
	fixedCap *int64

	mu      sync.Mutex
	index   map[string]*entry
	used    int64
	evicts  atomic.Int64
	enabled bool
	OnEvict func()
}

type entry struct {
	size       int64
	lastAccess time.Time
}

// Object is a cache hit.
type Object struct {
	Path    string
	Size    int64
	CType   string
	ModTime time.Time
	File    *os.File
}

type Stats struct {
	Enabled   bool
	Bytes     int64
	Objects   int64
	Budget    int64
	Evictions int64
	Dir       string
}

// New creates a disk cache. fixedCap nil means auto budget; *0 disables.
func New(root string, reserve int64, fixedCap *int64) (*Cache, error) {
	c := &Cache{
		root:     root,
		reserve:  reserve,
		fixedCap: fixedCap,
		index:    make(map[string]*entry),
	}
	if fixedCap != nil && *fixedCap == 0 {
		c.enabled = false
		slog.Info("Local disk cache disabled", "LOCAL_CACHE_BYTES", 0)
		return c, nil
	}
	if root == "" {
		c.enabled = false
		return c, nil
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("diskcache mkdir: %w", err)
	}
	c.enabled = true
	if err := c.rebuild(); err != nil {
		slog.Warn("diskcache rebuild incomplete", "err", err)
	}
	slog.Info("Local disk cache ready",
		"dir", root,
		"objects", len(c.index),
		"bytes", c.used,
		"budget", c.Budget(),
		"reserve", reserve,
	)
	return c, nil
}

func (c *Cache) Enabled() bool { return c != nil && c.enabled }

func (c *Cache) Stats() Stats {
	if c == nil || !c.enabled {
		return Stats{Enabled: false, Dir: ""}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return Stats{
		Enabled:   true,
		Bytes:     c.used,
		Objects:   int64(len(c.index)),
		Budget:    c.budgetLocked(),
		Evictions: c.evicts.Load(),
		Dir:       c.root,
	}
}

func (c *Cache) Budget() int64 {
	if c == nil || !c.enabled {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.budgetLocked()
}

func (c *Cache) budgetLocked() int64 {
	if c.fixedCap != nil {
		if *c.fixedCap < 0 {
			return 0
		}
		return *c.fixedCap
	}
	free := freeBytes(c.root)
	b := free - c.reserve
	// Account for space already used by the cache sitting on the same FS:
	// free does not include our used bytes; budget for cache content is used+allocatable.
	b = c.used + b
	if b < 0 {
		return 0
	}
	return b
}

func freeBytes(path string) int64 {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0
	}
	return int64(st.Bavail) * int64(st.Bsize)
}

func (c *Cache) objectPath(key string) (string, error) {
	if key == "" || strings.Contains(key, "\x00") || strings.HasPrefix(key, "/") {
		return "", fmt.Errorf("invalid key")
	}
	clean := filepath.Clean(key)
	if clean == "." || strings.HasPrefix(clean, "..") || strings.Contains(clean, string(filepath.Separator)+"..") {
		return "", fmt.Errorf("invalid key path")
	}
	full := filepath.Join(c.root, clean)
	rel, err := filepath.Rel(c.root, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("key escapes root")
	}
	return full, nil
}

func ctypePath(objectPath string) string { return objectPath + ".ctype" }

func (c *Cache) rebuild() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.index = make(map[string]*entry)
	c.used = 0
	return filepath.WalkDir(c.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".ctype") {
			return nil
		}
		rel, err := filepath.Rel(c.root, path)
		if err != nil {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		key := filepath.ToSlash(rel)
		c.index[key] = &entry{size: info.Size(), lastAccess: info.ModTime()}
		c.used += info.Size()
		return nil
	})
}

// Lookup opens a cached object if present. Caller must Close File.
func (c *Cache) Lookup(key string) (*Object, error) {
	if !c.Enabled() {
		return nil, os.ErrNotExist
	}
	full, err := c.objectPath(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(full)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	ctype := "application/octet-stream"
	if b, err := os.ReadFile(ctypePath(full)); err == nil {
		s := strings.TrimSpace(string(b))
		if s != "" {
			ctype = s
		}
	}
	c.mu.Lock()
	if e, ok := c.index[key]; ok {
		e.lastAccess = time.Now()
		e.size = st.Size()
	} else {
		c.index[key] = &entry{size: st.Size(), lastAccess: time.Now()}
		c.used += st.Size()
	}
	c.mu.Unlock()
	return &Object{
		Path:    full,
		Size:    st.Size(),
		CType:   ctype,
		ModTime: st.ModTime(),
		File:    f,
	}, nil
}

// Delete removes a key from disk and the index.
func (c *Cache) Delete(key string) {
	if !c.Enabled() {
		return
	}
	full, err := c.objectPath(key)
	if err != nil {
		return
	}
	_ = os.Remove(full)
	_ = os.Remove(ctypePath(full))
	c.mu.Lock()
	if e, ok := c.index[key]; ok {
		c.used -= e.size
		if c.used < 0 {
			c.used = 0
		}
		delete(c.index, key)
	}
	c.mu.Unlock()
}

// PutFile installs path into the cache (copy or rename). size should match file size.
func (c *Cache) PutFile(key, contentType, srcPath string, size int64) error {
	if !c.Enabled() {
		return nil
	}
	if size < 0 {
		return fmt.Errorf("negative size")
	}
	full, err := c.objectPath(key)
	if err != nil {
		return err
	}
	if err := c.ensureSpace(size, key); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	tmp := full + ".tmp"
	if err := copyFile(srcPath, tmp); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, full); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	ct := contentType
	if ct == "" {
		ct = "application/octet-stream"
	}
	_ = os.WriteFile(ctypePath(full), []byte(ct+"\n"), 0o644)

	c.mu.Lock()
	if e, ok := c.index[key]; ok {
		c.used -= e.size
	}
	c.index[key] = &entry{size: size, lastAccess: time.Now()}
	c.used += size
	c.mu.Unlock()
	return nil
}

// PutReader writes r (exactly size bytes) into the cache.
func (c *Cache) PutReader(key, contentType string, r io.Reader, size int64) error {
	if !c.Enabled() {
		return nil
	}
	full, err := c.objectPath(key)
	if err != nil {
		return err
	}
	if err := c.ensureSpace(size, key); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	tmp := full + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	n, copyErr := io.Copy(f, io.LimitReader(r, size))
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	if n != size {
		_ = os.Remove(tmp)
		return fmt.Errorf("short write: got %d want %d", n, size)
	}
	if err := os.Rename(tmp, full); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	ct := contentType
	if ct == "" {
		ct = "application/octet-stream"
	}
	_ = os.WriteFile(ctypePath(full), []byte(ct+"\n"), 0o644)
	c.mu.Lock()
	if e, ok := c.index[key]; ok {
		c.used -= e.size
	}
	c.index[key] = &entry{size: size, lastAccess: time.Now()}
	c.used += size
	c.mu.Unlock()
	return nil
}

func (c *Cache) ensureSpace(need int64, replaceKey string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	budget := c.budgetLocked()
	used := c.used
	if e, ok := c.index[replaceKey]; ok {
		used -= e.size
	}
	for used+need > budget {
		victim := c.lruVictimLocked(replaceKey)
		if victim == "" {
			return fmt.Errorf("disk cache full: need %d budget %d used %d", need, budget, used)
		}
		full, err := c.objectPath(victim)
		if err == nil {
			_ = os.Remove(full)
			_ = os.Remove(ctypePath(full))
		}
		if e, ok := c.index[victim]; ok {
			used -= e.size
			c.used -= e.size
			delete(c.index, victim)
			c.evicts.Add(1)
			if c.OnEvict != nil {
				c.OnEvict()
			}
		}
		budget = c.budgetLocked()
	}
	// Also respect reserve: if free disk would drop below reserve after write
	free := freeBytes(c.root)
	if free-need < c.reserve && c.fixedCap == nil {
		for free-need < c.reserve {
			victim := c.lruVictimLocked(replaceKey)
			if victim == "" {
				return fmt.Errorf("insufficient free disk for cache write")
			}
			full, err := c.objectPath(victim)
			if err == nil {
				_ = os.Remove(full)
				_ = os.Remove(ctypePath(full))
			}
			if e, ok := c.index[victim]; ok {
				c.used -= e.size
				delete(c.index, victim)
				c.evicts.Add(1)
				if c.OnEvict != nil {
					c.OnEvict()
				}
			}
			free = freeBytes(c.root)
		}
	}
	return nil
}

func (c *Cache) lruVictimLocked(except string) string {
	var bestKey string
	var bestTime time.Time
	first := true
	for k, e := range c.index {
		if k == except {
			continue
		}
		if first || e.lastAccess.Before(bestTime) {
			bestKey = k
			bestTime = e.lastAccess
			first = false
		}
	}
	return bestKey
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
