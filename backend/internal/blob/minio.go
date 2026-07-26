package blob

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Store interface {
	Put(context.Context, string, []byte, string) error
	Get(context.Context, string) ([]byte, error)
	Delete(context.Context, string) error
}
type Noop struct{}

func (Noop) Put(context.Context, string, []byte, string) error { return nil }
func (Noop) Get(context.Context, string) ([]byte, error) {
	return nil, errors.New("object storage is not configured")
}
func (Noop) Delete(context.Context, string) error { return nil }

type MinIO struct {
	client *minio.Client
	bucket string
}

func NewMinIO(ctx context.Context, endpoint, accessKey, secretKey, bucket string, secure bool) (*MinIO, error) {
	if endpoint == "" {
		return nil, errors.New("minio endpoint is empty")
	}
	client, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(accessKey, secretKey, ""), Secure: secure})
	if err != nil {
		return nil, err
	}
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, err
	}
	if !exists {
		if err = client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("create minio bucket: %w", err)
		}
	}
	return &MinIO{client: client, bucket: bucket}, nil
}
func (m *MinIO) Put(ctx context.Context, key string, data []byte, mime string) error {
	_, err := m.client.PutObject(ctx, m.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: mime})
	return err
}

func (m *MinIO) Get(ctx context.Context, key string) ([]byte, error) {
	object, err := m.client.GetObject(ctx, m.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer object.Close()
	if _, err = object.Stat(); err != nil {
		return nil, err
	}
	return io.ReadAll(io.LimitReader(object, 32<<20))
}

func (m *MinIO) Delete(ctx context.Context, key string) error {
	return m.client.RemoveObject(ctx, m.bucket, key, minio.RemoveObjectOptions{})
}
