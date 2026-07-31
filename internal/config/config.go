package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AWSAccessKeyID     string
	AWSSecretAccessKey string
	S3Endpoint         string
	S3Bucket           string
	S3Region           string
	S3Addressing       string // path | virtual
	S3QuotaBytes       *int64
	S3UsageRefresh     time.Duration

	UpstreamTimeout       time.Duration
	MetadataCacheSeconds  int
	PackageCacheSeconds   int
	NegativeCacheSeconds  int
	MaxSpoolBytes         int64
	MaxConcurrentSpools   int
	MinTmpFreeBytes       int64
	MaxUpstreamErrorBytes int64
	HeadCacheMax          int

	// Local disk cache: Dir empty disables. Bytes nil = auto; *0 = off; >0 = fixed cap.
	LocalCacheDir           string
	LocalCacheReserveBytes  int64
	LocalCacheBytes         *int64

	MetricsStatePath    string
	MetricsFlushSeconds time.Duration

	LogLevel string
	Listen   string

	WebRoot     string // templates + static parent (web/)
	DocsRoot    string
	ScriptsRoot string
}

func Load() (*Config, error) {
	access := os.Getenv("AWS_ACCESS_KEY_ID")
	secret := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if access == "" || secret == "" {
		return nil, fmt.Errorf("AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY are required")
	}

	cfg := &Config{
		AWSAccessKeyID:        access,
		AWSSecretAccessKey:    secret,
		S3Endpoint:            strings.TrimRight(getenv("S3_ENDPOINT", "https://s3.scalewithus.com"), "/"),
		S3Bucket:              getenv("S3_BUCKET", "linux-mirrors"),
		S3Region:              getenv("S3_REGION", "us-east-1"),
		S3Addressing:          getenv("S3_ADDRESSING", "path"),
		S3UsageRefresh:        time.Duration(getenvInt("S3_USAGE_REFRESH_SECONDS", 300)) * time.Second,
		UpstreamTimeout:       time.Duration(getenvFloat("UPSTREAM_TIMEOUT", 120)) * time.Second,
		MetadataCacheSeconds:  getenvInt("METADATA_CACHE_SECONDS", 21600),
		PackageCacheSeconds:   getenvInt("PACKAGE_CACHE_SECONDS", 15552000),
		NegativeCacheSeconds:  getenvInt("NEGATIVE_CACHE_SECONDS", 60),
		MaxSpoolBytes:         getenvInt64("MAX_SPOOL_BYTES", 2*1024*1024*1024),
		MaxConcurrentSpools:   getenvInt("MAX_CONCURRENT_SPOOLS", 3),
		MinTmpFreeBytes:       getenvInt64("MIN_TMP_FREE_BYTES", 512*1024*1024),
		MaxUpstreamErrorBytes: getenvInt64("MAX_UPSTREAM_ERROR_BYTES", 64*1024),
		HeadCacheMax:           getenvInt("HEAD_CACHE_MAX", 50000),
		LocalCacheDir:          getenv("LOCAL_CACHE_DIR", "/app/data/cache"),
		LocalCacheReserveBytes: getenvInt64("LOCAL_CACHE_RESERVE_BYTES", 15*1024*1024*1024),
		MetricsStatePath:       getenv("METRICS_STATE_PATH", "/var/lib/linux-mirror/metrics.json"),
		MetricsFlushSeconds:    time.Duration(getenvFloat("METRICS_FLUSH_SECONDS", 10)) * time.Second,
		LogLevel:               strings.ToUpper(getenv("LOG_LEVEL", "INFO")),
		Listen:                 getenv("LISTEN", ":8080"),
		WebRoot:                getenv("WEB_ROOT", "web"),
		DocsRoot:               getenv("DOCS_ROOT", "docs"),
		ScriptsRoot:            getenv("SCRIPTS_ROOT", "scripts"),
	}
	if raw := strings.TrimSpace(os.Getenv("S3_QUOTA_BYTES")); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("S3_QUOTA_BYTES: %w", err)
		}
		cfg.S3QuotaBytes = &v
	}
	// LOCAL_CACHE_BYTES: unset/empty = auto; 0 = disabled; >0 = fixed cap
	if raw := strings.TrimSpace(os.Getenv("LOCAL_CACHE_BYTES")); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("LOCAL_CACHE_BYTES: %w", err)
		}
		cfg.LocalCacheBytes = &v
	}
	if cfg.MaxConcurrentSpools < 1 {
		cfg.MaxConcurrentSpools = 1
	}
	return cfg, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getenvInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func getenvInt64(k string, def int64) int64 {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}

func getenvFloat(k string, def float64) float64 {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return n
}
