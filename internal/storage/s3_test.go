package storage_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

func TestS3Client_UploadUsesCustomEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/recordings-bucket/recordings/1.ogg", r.URL.Path)
		assert.Equal(t, "audio/ogg", r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.Equal(t, "recording", string(body))
	}))
	defer server.Close()

	client, err := storage.NewS3Client(&config.StorageConfig{
		S3Bucket:       "recordings-bucket",
		S3Region:       "auto",
		S3Key:          "test-key",
		S3Secret:       "test-secret",
		S3Endpoint:     server.URL,
		S3UsePathStyle: true,
	})
	require.NoError(t, err)

	err = client.Upload(t.Context(), "recordings/1.ogg", strings.NewReader("recording"), "audio/ogg")
	require.NoError(t, err)
}

func TestS3Client_PresignedURLAddressing(t *testing.T) {
	t.Setenv("AWS_IGNORE_CONFIGURED_ENDPOINT_URLS", "true")
	tests := []struct {
		name         string
		endpoint     string
		usePathStyle bool
		wantHost     string
		wantPath     string
	}{
		{
			name:     "AWS default endpoint",
			wantHost: "recordings-bucket.s3.us-east-1.amazonaws.com",
			wantPath: "/recordings/1.ogg",
		},
		{
			name:     "custom endpoint",
			endpoint: "https://storage.example.com",
			wantHost: "recordings-bucket.storage.example.com",
			wantPath: "/recordings/1.ogg",
		},
		{
			name:         "custom endpoint with path style",
			endpoint:     "https://storage.example.com",
			usePathStyle: true,
			wantHost:     "storage.example.com",
			wantPath:     "/recordings-bucket/recordings/1.ogg",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := storage.NewS3Client(&config.StorageConfig{
				S3Bucket:       "recordings-bucket",
				S3Region:       "us-east-1",
				S3Key:          "test-key",
				S3Secret:       "test-secret",
				S3Endpoint:     tt.endpoint,
				S3UsePathStyle: tt.usePathStyle,
			})
			require.NoError(t, err)

			signedURL, err := client.GetPresignedURL(t.Context(), "recordings/1.ogg", 15*time.Minute)
			require.NoError(t, err)
			parsed, err := url.Parse(signedURL)
			require.NoError(t, err)
			assert.Equal(t, tt.wantHost, parsed.Host)
			assert.Equal(t, tt.wantPath, parsed.Path)
			assert.NotEmpty(t, parsed.Query().Get("X-Amz-Signature"))
		})
	}
}
