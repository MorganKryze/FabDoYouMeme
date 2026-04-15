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
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
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

// Purge deletes every object under prefix. Uses ListObjectsV2Pager +
// batched DeleteObjects (max 1000 keys per batch, the S3 limit). Returns
// the running total on error so the caller can surface partial progress
// in audit logs. Passing "" empties the entire bucket.
func (s *S3Storage) Purge(ctx context.Context, prefix string) (int64, error) {
	var total int64
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return total, fmt.Errorf("purge list: %w", err)
		}
		if len(page.Contents) == 0 {
			continue
		}
		ids := make([]s3types.ObjectIdentifier, 0, len(page.Contents))
		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}
			ids = append(ids, s3types.ObjectIdentifier{Key: obj.Key})
		}
		if len(ids) == 0 {
			continue
		}
		_, err = s.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(s.bucket),
			Delete: &s3types.Delete{Objects: ids, Quiet: aws.Bool(true)},
		})
		if err != nil {
			return total, fmt.Errorf("purge delete batch: %w", err)
		}
		total += int64(len(ids))
	}
	return total, nil
}

// Stats walks every object under prefix and returns the object count and
// aggregate byte size. Uses the same paginator strategy as Purge; failure
// returns the running totals so the caller can still surface partial data.
func (s *S3Storage) Stats(ctx context.Context, prefix string) (int64, int64, error) {
	var count, bytes int64
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return count, bytes, fmt.Errorf("stats list: %w", err)
		}
		for _, obj := range page.Contents {
			count++
			if obj.Size != nil {
				bytes += *obj.Size
			}
		}
	}
	return count, bytes, nil
}

// Compile-time interface check
var _ Storage = (*S3Storage)(nil)
