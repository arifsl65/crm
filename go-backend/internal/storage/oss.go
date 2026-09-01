// Package storage provides cloud storage operations using Alibaba Cloud OSS.
package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/config"
)

// OSSClient provides operations for Alibaba Cloud OSS.
type OSSClient struct {
	client *oss.Client
	bucket *oss.Bucket
	config config.OSSConfig
}

// NewOSSClient creates a new OSS client.
// Returns nil if OSS is not configured/enabled.
func NewOSSClient(cfg config.OSSConfig) (*OSSClient, error) {
	if !cfg.Enabled {
		log.Info().Msg("OSS storage disabled")
		return nil, nil
	}

	if cfg.AccessKeyID == "" || cfg.AccessKeySecret == "" {
		log.Warn().Msg("OSS credentials not configured, storage disabled")
		return nil, nil
	}

	// Create OSS client with V2 signature (V1 is disabled on this bucket)
	client, err := oss.New(cfg.Endpoint, cfg.AccessKeyID, cfg.AccessKeySecret,
		oss.AuthVersion(oss.AuthV2),
		oss.Timeout(30, 120), // 30s connect, 120s read/write
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OSS client: %w", err)
	}

	bucket, err := client.Bucket(cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to get OSS bucket: %w", err)
	}

	log.Info().
		Str("endpoint", cfg.Endpoint).
		Str("bucket", cfg.Bucket).
		Msg("OSS storage initialized")

	return &OSSClient{
		client: client,
		bucket: bucket,
		config: cfg,
	}, nil
}

// Upload uploads a file to OSS.
// Returns the object key (path) in OSS.
func (c *OSSClient) Upload(ctx context.Context, key string, data []byte, contentType string) error {
	if c == nil {
		return fmt.Errorf("OSS client not initialized")
	}

	options := []oss.Option{
		oss.ContentType(contentType),
		oss.ObjectACL(oss.ACLPrivate),
	}

	reader := bytes.NewReader(data)

	start := time.Now()
	err := c.bucket.PutObject(key, reader, options...)
	if err != nil {
		log.Error().
			Err(err).
			Str("key", key).
			Int("size", len(data)).
			Msg("Failed to upload to OSS")
		return fmt.Errorf("OSS upload failed: %w", err)
	}

	log.Info().
		Str("key", key).
		Int("size", len(data)).
		Str("content_type", contentType).
		Dur("duration", time.Since(start)).
		Msg("Uploaded to OSS")

	return nil
}

// Download downloads a file from OSS.
func (c *OSSClient) Download(ctx context.Context, key string) ([]byte, error) {
	if c == nil {
		return nil, fmt.Errorf("OSS client not initialized")
	}

	start := time.Now()
	result, err := c.bucket.GetObject(key)
	if err != nil {
		log.Error().
			Err(err).
			Str("key", key).
			Msg("Failed to download from OSS")
		return nil, fmt.Errorf("OSS download failed: %w", err)
	}
	defer result.Close()

	data, err := io.ReadAll(result)
	if err != nil {
		return nil, fmt.Errorf("failed to read OSS object: %w", err)
	}

	log.Info().
		Str("key", key).
		Int("size", len(data)).
		Dur("duration", time.Since(start)).
		Msg("Downloaded from OSS")

	return data, nil
}

// Delete deletes a file from OSS.
func (c *OSSClient) Delete(ctx context.Context, key string) error {
	if c == nil {
		return fmt.Errorf("OSS client not initialized")
	}

	err := c.bucket.DeleteObject(key)
	if err != nil {
		log.Error().
			Err(err).
			Str("key", key).
			Msg("Failed to delete from OSS")
		return fmt.Errorf("OSS delete failed: %w", err)
	}

	log.Info().
		Str("key", key).
		Msg("Deleted from OSS")

	return nil
}

// Exists checks if a file exists in OSS.
func (c *OSSClient) Exists(ctx context.Context, key string) (bool, error) {
	if c == nil {
		return false, fmt.Errorf("OSS client not initialized")
	}

	exists, err := c.bucket.IsObjectExist(key)
	if err != nil {
		return false, fmt.Errorf("OSS exists check failed: %w", err)
	}

	return exists, nil
}

// GetSignedURL generates a signed URL for temporary access to a private object.
func (c *OSSClient) GetSignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	if c == nil {
		return "", fmt.Errorf("OSS client not initialized")
	}

	signedURL, err := c.bucket.SignURL(key, oss.HTTPGet, int64(expiry.Seconds()))
	if err != nil {
		return "", fmt.Errorf("failed to generate signed URL: %w", err)
	}

	return signedURL, nil
}

// IsConfigured returns true if OSS is properly configured and ready to use.
func (c *OSSClient) IsConfigured() bool {
	return c != nil && c.bucket != nil
}
