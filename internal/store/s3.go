package store

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"github.com/Scalewithus-India/linux-mirror-cashing-server/internal/config"
)

type ObjectHead struct {
	ContentLength int64
	ContentType   string
	LastModified  time.Time
}

// GetResult is a GetObject response with enough metadata to serve without a prior Head.
type GetResult struct {
	Body          io.ReadCloser
	ContentLength int64 // bytes in this response body
	ContentType   string
	LastModified  time.Time
	ContentRange  string
	StatusCode    int // 200 or 206
	ObjectSize    int64 // full object size when known
}

type Usage struct {
	mu          sync.Mutex
	UsedBytes   int64
	ObjectCount int64
	RefreshedAt float64
	Error       string
}

func (u *Usage) Snapshot(quota *int64) map[string]any {
	u.mu.Lock()
	defer u.mu.Unlock()
	var free any
	var quotaOut any
	if quota != nil {
		quotaOut = *quota
		f := *quota - u.UsedBytes
		if f < 0 {
			f = 0
		}
		free = f
	}
	var refreshed any
	if u.RefreshedAt > 0 {
		refreshed = u.RefreshedAt
	}
	var errOut any
	if u.Error != "" {
		errOut = u.Error
	}
	return map[string]any{
		"s3_used_bytes":         u.UsedBytes,
		"s3_object_count":       u.ObjectCount,
		"s3_quota_bytes":        quotaOut,
		"s3_free_bytes":         free,
		"s3_usage_refreshed_at": refreshed,
		"s3_usage_error":        errOut,
	}
}

func (u *Usage) NoteStore(size int64, replaced *int64) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if replaced == nil {
		u.ObjectCount++
		u.UsedBytes += size
	} else {
		u.UsedBytes += size - *replaced
	}
}

type Store struct {
	Client *s3.Client
	Bucket string
	Usage  Usage
	Quota  *int64
}

func NewHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          256,
			MaxIdleConnsPerHost:   64,
			MaxConnsPerHost:       0,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			ResponseHeaderTimeout: 60 * time.Second,
		},
	}
}

func New(ctx context.Context, cfg *config.Config) (*Store, error) {
	httpClient := NewHTTPClient()
	awsCfg := aws.Config{
		Region: cfg.S3Region,
		Credentials: credentials.NewStaticCredentialsProvider(
			cfg.AWSAccessKeyID, cfg.AWSSecretAccessKey, "",
		),
		HTTPClient: httpClient,
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.S3Endpoint)
		o.UsePathStyle = cfg.S3Addressing == "path"
	})
	st := &Store{Client: client, Bucket: cfg.S3Bucket, Quota: cfg.S3QuotaBytes}
	_, err := client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(cfg.S3Bucket)})
	if err != nil {
		slog.Warn("S3 head_bucket failed (will still try per-object)", "err", err)
	} else {
		slog.Info("S3 OK", "endpoint", cfg.S3Endpoint, "bucket", cfg.S3Bucket)
	}
	return st, nil
}

func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	var nsk *types.NotFound
	if errors.As(err, &nsk) {
		return true
	}
	var nsk2 *types.NoSuchKey
	if errors.As(err, &nsk2) {
		return true
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey", "404", "NoSuchBucket":
			return true
		}
	}
	return false
}

func IsInvalidRange(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return apiErr.ErrorCode() == "InvalidRange"
	}
	return false
}

func (s *Store) Head(ctx context.Context, key string) (*ObjectHead, error) {
	out, err := s.Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	h := &ObjectHead{}
	if out.ContentLength != nil {
		h.ContentLength = *out.ContentLength
	}
	if out.ContentType != nil {
		h.ContentType = *out.ContentType
	}
	if out.LastModified != nil {
		h.LastModified = out.LastModified.UTC()
	}
	return h, nil
}

func (s *Store) Get(ctx context.Context, key string, byteRange string) (*GetResult, error) {
	in := &s3.GetObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(key),
	}
	if byteRange != "" {
		in.Range = aws.String(byteRange)
	}
	out, err := s.Client.GetObject(ctx, in)
	if err != nil {
		return nil, err
	}
	res := &GetResult{
		Body:       out.Body,
		StatusCode: http.StatusOK,
	}
	if out.ContentLength != nil {
		res.ContentLength = *out.ContentLength
		res.ObjectSize = *out.ContentLength
	}
	if out.ContentType != nil {
		res.ContentType = *out.ContentType
	}
	if out.LastModified != nil {
		res.LastModified = out.LastModified.UTC()
	}
	if out.ContentRange != nil && *out.ContentRange != "" {
		res.ContentRange = *out.ContentRange
		res.StatusCode = http.StatusPartialContent
		// ContentLength is the part size; parse full size from Content-Range if present.
		if full, ok := parseContentRangeTotal(res.ContentRange); ok {
			res.ObjectSize = full
		}
	}
	return res, nil
}

func parseContentRangeTotal(cr string) (int64, bool) {
	// bytes start-end/total
	var start, end, total int64
	n, err := fmt.Sscanf(cr, "bytes %d-%d/%d", &start, &end, &total)
	if err != nil || n != 3 {
		return 0, false
	}
	return total, true
}

func (s *Store) PutFile(ctx context.Context, key, contentType, path string, size int64) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = s.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.Bucket),
		Key:           aws.String(key),
		Body:          f,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
	})
	return err
}

func (s *Store) RefreshUsage(ctx context.Context) error {
	var used, count int64
	var token *string
	for {
		out, err := s.Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(s.Bucket),
			ContinuationToken: token,
			MaxKeys:           aws.Int32(1000),
		})
		if err != nil {
			s.Usage.mu.Lock()
			s.Usage.Error = err.Error()
			s.Usage.mu.Unlock()
			return err
		}
		for _, obj := range out.Contents {
			count++
			if obj.Size != nil {
				used += *obj.Size
			}
		}
		if !aws.ToBool(out.IsTruncated) {
			break
		}
		token = out.NextContinuationToken
	}
	s.Usage.mu.Lock()
	s.Usage.UsedBytes = used
	s.Usage.ObjectCount = count
	s.Usage.RefreshedAt = float64(time.Now().Unix()) + float64(time.Now().Nanosecond())/1e9
	s.Usage.Error = ""
	s.Usage.mu.Unlock()
	slog.Info("S3 usage refreshed", "objects", count, "bytes", used)
	return nil
}

func (s *Store) UsageLoop(ctx context.Context, every time.Duration) {
	if every < 30*time.Second {
		every = 30 * time.Second
	}
	for {
		if err := s.RefreshUsage(ctx); err != nil {
			slog.Warn("S3 usage refresh failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(every):
		}
	}
}

func FormatBucketURL(bucket, key string) string {
	return fmt.Sprintf("s3://%s/%s", bucket, key)
}
