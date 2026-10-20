package storage_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestS3Client_CustomEndpoints(t *testing.T) {
	var uploaded string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/recordings/calls/test.wav", r.URL.Path)
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		uploaded = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	c, err := storage.NewS3Client(&config.StorageConfig{
		S3Bucket: "recordings", S3Region: "us-east-1",
		S3Key: "test", S3Secret: "test-secret",
		S3Endpoint: server.URL, S3PublicEndpoint: "http://localhost:9000",
		S3UsePathStyle: true,
	})
	require.NoError(t, err)
	require.NoError(t, c.Upload(context.Background(), "calls/test.wav", strings.NewReader("audio"), "audio/wav"))
	assert.Equal(t, "audio", uploaded)
	signed, err := c.GetPresignedURL(context.Background(), "calls/test.wav", 15*time.Minute)
	require.NoError(t, err)
	u, err := url.Parse(signed)
	require.NoError(t, err)
	assert.Equal(t, "localhost:9000", u.Host)
	assert.Equal(t, "/recordings/calls/test.wav", u.Path)
	assert.NotEmpty(t, u.Query().Get("X-Amz-Signature"))
	assert.Equal(t, "900", u.Query().Get("X-Amz-Expires"))
}

// Run explicitly against the local Docker S3 service.
func TestS3Client_LocalIntegration(t *testing.T) {
	endpoint := os.Getenv("WHATOMATE_TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("set WHATOMATE_TEST_S3_ENDPOINT to run the local S3 integration test")
	}
	c, err := storage.NewS3Client(&config.StorageConfig{
		S3Bucket: "whatomate-recordings", S3Region: "us-east-1",
		S3Key: "whatomate-local", S3Secret: "whatomate-local-dev-secret",
		S3Endpoint: endpoint, S3UsePathStyle: true,
	})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	const key = "diagnostics/s3-test.txt"
	require.NoError(t, c.Upload(ctx, key, strings.NewReader("whatomate local S3 test"), "text/plain"))
	signed, err := c.GetPresignedURL(ctx, key, time.Minute)
	require.NoError(t, err)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, signed, nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "whatomate local S3 test", string(body))
}

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
