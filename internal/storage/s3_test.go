package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NewS3Client validation: bucket and region required.

func TestNewS3Client_RequiresBucket(t *testing.T) {
	_, err := storage.NewS3Client(&config.StorageConfig{
		S3Region: "us-east-1",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "s3_bucket")
}

func TestNewS3Client_RequiresRegion(t *testing.T) {
	_, err := storage.NewS3Client(&config.StorageConfig{
		S3Bucket: "my-bucket",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "s3_region")
}

func TestNewS3Client_Succeeds_WithBucketAndRegion(t *testing.T) {
	c, err := storage.NewS3Client(&config.StorageConfig{
		S3Bucket: "b",
		S3Region: "us-east-1",
	})
	require.NoError(t, err)
	assert.NotNil(t, c)
}

func TestNewS3Client_AcceptsStaticCredentials(t *testing.T) {
	c, err := storage.NewS3Client(&config.StorageConfig{
		S3Bucket: "b",
		S3Region: "us-east-1",
		S3Key:    "AKIA...",
		S3Secret: "secret",
	})
	require.NoError(t, err)
	assert.NotNil(t, c)
}

// Without s3_key/s3_secret the AWS default credential chain is used, so an
// attached IAM role (or, here, the environment) supplies the credentials.
func TestNewS3Client_UsesDefaultCredentialChain(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAFROMENV")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "envsecret")

	c, err := storage.NewS3Client(&config.StorageConfig{
		S3Bucket: "b",
		S3Region: "us-east-1",
	})
	require.NoError(t, err)

	url, err := c.GetPresignedURL(context.Background(), "recordings/1.wav", 15*time.Minute)
	require.NoError(t, err)
	assert.Contains(t, url, "AKIAFROMENV")
}
