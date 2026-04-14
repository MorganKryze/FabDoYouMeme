// backend/internal/storage/s3.go
package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Storage is the concrete Storage implementation backed by an S3-compatible store (RustFS).
type S3Storage struct {
	client    *s3.Client
	presigner *s3.PresignClient
	bucket    string
}

// NewS3 builds an S3Storage pointed at the given endpoint.
// UsePathStyle is forced on — required by RustFS.
func NewS3(endpoint, accessKey, secretKey, bucket string) (*S3Storage, error) {
	if endpoint == "" || accessKey == "" || secretKey == "" || bucket == "" {
		return nil, fmt.Errorf("storage: endpoint, accessKey, secretKey, and bucket are required")
	}

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion("us-east-1"), // RustFS ignores region value
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("storage: load AWS config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true // RustFS requires path-style, not virtual-hosted-style
	})

	return &S3Storage{
		client:    client,
		presigner: s3.NewPresignClient(client),
		bucket:    bucket,
	}, nil
}

// PresignUpload returns a pre-signed PUT URL valid for ttl.
func (s *S3Storage) PresignUpload(ctx context.Context, key string, ttl time.Duration) (string, error) {
	req, err := s.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("presign upload %q: %w", key, err)
	}
	return req.URL, nil
}

// PresignDownload returns a pre-signed GET URL with attachment disposition.
func (s *S3Storage) PresignDownload(ctx context.Context, key string, ttl time.Duration) (string, error) {
	req, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket:                     aws.String(s.bucket),
		Key:                        aws.String(key),
		ResponseContentDisposition: aws.String("attachment"),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("presign download %q: %w", key, err)
	}
	return req.URL, nil
}

// Upload streams body to s3://bucket/key with the given content type. The
// caller MUST pass the exact byte count so the SDK can send Content-Length
// and avoid chunked transfer (RustFS is stricter than AWS on this).
func (s *S3Storage) Upload(ctx context.Context, key string, body io.Reader, contentType string, size int64) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	})
	if err != nil {
		return fmt.Errorf("upload %q: %w", key, err)
	}
	return nil
}

// Download fetches the object at key and returns a stream plus its metadata.
// The caller is responsible for closing the returned body.
func (s *S3Storage) Download(ctx context.Context, key string) (io.ReadCloser, string, int64, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, "", 0, fmt.Errorf("download %q: %w", key, err)
	}
	contentType := ""
	if out.ContentType != nil {
		contentType = *out.ContentType
	}
	var size int64
	if out.ContentLength != nil {
		size = *out.ContentLength
	}
	return out.Body, contentType, size, nil
}

// Delete removes the object at key. Returns nil if the key does not exist.
func (s *S3Storage) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete %q: %w", key, err)
	}
	return nil
}

// Probe checks that the bucket is accessible by calling HeadBucket.
func (s *S3Storage) Probe(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)})
	if err != nil {
		return fmt.Errorf("storage probe: %w", err)
	}
	return nil
}

// Compile-time interface check
var _ Storage = (*S3Storage)(nil)
