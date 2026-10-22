package transcribe

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// The recording archive (docs/v2-intelligence-report.md §4.8): a private bucket on an S3-compatible
// store (Backblaze B2 preferred, Cloudflare R2 fallback), credentials in the server env only, with an
// application key limited to this one bucket. Objects are content-addressed:
//
//	recordings/{tenant_id}/{platform}/{yyyy}/{mm}/{sha256}.{ext}
//
// so a re-upload is a no-op, and no phone number or call id is in the key. Server-side encryption
// (SSE, AES-256) is requested on every upload. Retention: a bucket lifecycle rule on recordings/ and
// the Calyx cleanup job (cleanup.go).

type ObjectStore interface {
	Exists(ctx context.Context, key string) (bool, error)
	Put(ctx context.Context, key string, body []byte, contentType string) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}

// ObjectKey is the archive key for one recording.
func ObjectKey(tenantID, platform string, downloadedAt time.Time, sha256hex, contentType string) string {
	ext := "mp3"
	switch {
	case strings.Contains(contentType, "wav"):
		ext = "wav"
	case strings.Contains(contentType, "ogg"):
		ext = "ogg"
	}
	t := downloadedAt.UTC()
	return fmt.Sprintf("recordings/%s/%s/%04d/%02d/%s.%s", tenantID, platform, t.Year(), int(t.Month()), sha256hex, ext)
}

type S3Config struct {
	Endpoint  string // e.g. https://s3.us-west-004.backblazeb2.com
	Region    string // e.g. us-west-004
	Bucket    string
	KeyID     string
	SecretKey string
}

func (c S3Config) Configured() bool {
	return c.Endpoint != "" && c.Bucket != "" && c.KeyID != "" && c.SecretKey != ""
}

type S3Store struct {
	client *s3.Client
	bucket string
}

func NewS3Store(cfg S3Config) *S3Store {
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}
	client := s3.New(s3.Options{
		Region:       region,
		BaseEndpoint: aws.String(cfg.Endpoint),
		UsePathStyle: true,
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.KeyID, cfg.SecretKey, ""),
		HTTPClient:   &http.Client{Timeout: 5 * time.Minute},
		// Newer SDKs add CRC checksums to every request by default; B2 and R2 do not accept all of
		// them. Send checksums only where the S3 API requires them.
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	})
	return &S3Store{client: client, bucket: cfg.Bucket}
}

func (s *S3Store) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err == nil {
		return true, nil
	}
	var nf *types.NotFound
	var api smithy.APIError
	if errors.As(err, &nf) || (errors.As(err, &api) && (api.ErrorCode() == "NotFound" || api.ErrorCode() == "NoSuchKey")) {
		return false, nil
	}
	return false, archiveErr("head", err)
}

func (s *S3Store) Put(ctx context.Context, key string, body []byte, contentType string) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:               aws.String(s.bucket),
		Key:                  aws.String(key),
		Body:                 bytes.NewReader(body),
		ContentLength:        aws.Int64(int64(len(body))),
		ContentType:          aws.String(contentType),
		ServerSideEncryption: types.ServerSideEncryptionAes256,
	})
	return archiveErr("put", err)
}

func (s *S3Store) Get(ctx context.Context, key string) ([]byte, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		return nil, archiveErr("get", err)
	}
	defer out.Body.Close()
	data, err := io.ReadAll(io.LimitReader(out.Body, MaxAudioBytes+1))
	if err != nil {
		return nil, archiveErr("get", err)
	}
	return data, nil
}

func (s *S3Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	return archiveErr("delete", err)
}

// archiveErr keeps the S3 error code and drops the rest: no endpoint URL, no key id.
func archiveErr(op string, err error) error {
	if err == nil {
		return nil
	}
	var api smithy.APIError
	if errors.As(err, &api) {
		return fmt.Errorf("archive %s: %s", op, api.ErrorCode())
	}
	return fmt.Errorf("archive %s: %s", op, safeErr(err))
}

// MemoryStore is the fake object store for tests (and the laptop fake-providers run).
type MemoryStore struct {
	Objects map[string][]byte
	Types   map[string]string
	FailPut error
	Puts    int
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{Objects: map[string][]byte{}, Types: map[string]string{}}
}

func (m *MemoryStore) Exists(_ context.Context, key string) (bool, error) {
	_, ok := m.Objects[key]
	return ok, nil
}

func (m *MemoryStore) Put(_ context.Context, key string, body []byte, contentType string) error {
	m.Puts++
	if m.FailPut != nil {
		return m.FailPut
	}
	m.Objects[key] = append([]byte(nil), body...)
	m.Types[key] = contentType
	return nil
}

func (m *MemoryStore) Get(_ context.Context, key string) ([]byte, error) {
	b, ok := m.Objects[key]
	if !ok {
		return nil, fmt.Errorf("archive get: NoSuchKey")
	}
	return append([]byte(nil), b...), nil
}

func (m *MemoryStore) Delete(_ context.Context, key string) error {
	delete(m.Objects, key)
	return nil
}
