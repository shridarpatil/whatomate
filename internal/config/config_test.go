package config_test

import (
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // matches the coturn TURN REST API derivation under test
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}

func TestLoad_AppliesDefaultsForMissingFields(t *testing.T) {
	cfg, err := config.Load(writeConfig(t, ""))
	require.NoError(t, err)

	assert.Equal(t, "Whatomate", cfg.App.Name)
	assert.Equal(t, "development", cfg.App.Environment)
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, 30, cfg.Server.ReadTimeout)
	assert.Equal(t, 30, cfg.Server.WriteTimeout)
	assert.Equal(t, 5432, cfg.Database.Port)
	assert.Equal(t, "disable", cfg.Database.SSLMode)
	assert.Equal(t, 25, cfg.Database.MaxOpenConns)
	assert.Equal(t, 5, cfg.Database.MaxIdleConns)
	assert.Equal(t, 300, cfg.Database.ConnMaxLifetime)
	assert.Equal(t, 6379, cfg.Redis.Port)
	assert.Equal(t, 15, cfg.JWT.AccessExpiryMins)
	assert.Equal(t, 1, cfg.JWT.RefreshExpiryDays)
	assert.Equal(t, "v18.0", cfg.WhatsApp.APIVersion)
	assert.Equal(t, "https://graph.facebook.com", cfg.WhatsApp.BaseURL)
	assert.Equal(t, "local", cfg.Storage.Type)
	assert.Equal(t, "./uploads", cfg.Storage.LocalPath)
	assert.Equal(t, "admin@admin.com", cfg.DefaultAdmin.Email)
	assert.Equal(t, "admin", cfg.DefaultAdmin.Password)
}

func TestLoad_FileValuesOverrideDefaults(t *testing.T) {
	cfg, err := config.Load(writeConfig(t, `
[app]
name = "MyApp"
environment = "production"

[server]
port = 9090

[database]
host = "db.example.com"
port = 5433
user = "u"
password = "p"
name = "n"

[whatsapp]
api_version = "v22.0"
`))
	require.NoError(t, err)

	assert.Equal(t, "MyApp", cfg.App.Name)
	assert.Equal(t, "production", cfg.App.Environment)
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "db.example.com", cfg.Database.Host)
	assert.Equal(t, 5433, cfg.Database.Port)
	assert.Equal(t, "v22.0", cfg.WhatsApp.APIVersion)
}

func TestLoad_ProductionEnvironmentForcesSecureCookie(t *testing.T) {
	cfg, err := config.Load(writeConfig(t, `
[app]
environment = "production"

[cookie]
secure = false
`))
	require.NoError(t, err)
	assert.True(t, cfg.Cookie.Secure, "production environment must force Cookie.Secure=true")
}

func TestLoad_DevelopmentDoesNotForceSecureCookie(t *testing.T) {
	cfg, err := config.Load(writeConfig(t, `
[app]
environment = "development"

[cookie]
secure = false
`))
	require.NoError(t, err)
	assert.False(t, cfg.Cookie.Secure)
}

func TestLoad_EnvVarsOverrideFile(t *testing.T) {
	t.Setenv("WHATOMATE_DATABASE_HOST", "from-env")
	t.Setenv("WHATOMATE_SERVER_PORT", "1234")

	cfg, err := config.Load(writeConfig(t, `
[database]
host = "from-file"

[server]
port = 8080
`))
	require.NoError(t, err)
	assert.Equal(t, "from-env", cfg.Database.Host, "WHATOMATE_DATABASE_HOST must override file")
	assert.Equal(t, 1234, cfg.Server.Port, "WHATOMATE_SERVER_PORT must override file")
}

func TestLoad_EmptyConfigPathStillLoadsDefaults(t *testing.T) {
	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.Equal(t, "Whatomate", cfg.App.Name)
	assert.Equal(t, 8080, cfg.Server.Port)
}

func TestLoad_MissingFileReturnsError(t *testing.T) {
	_, err := config.Load("/nonexistent/path/config.toml")
	require.Error(t, err)
}

func TestLoad_RateLimitDefaults(t *testing.T) {
	cfg, err := config.Load(writeConfig(t, ""))
	require.NoError(t, err)
	assert.Equal(t, 10, cfg.RateLimit.LoginMaxAttempts)
	assert.Equal(t, 10, cfg.RateLimit.RegisterMaxAttempts)
	assert.Equal(t, 30, cfg.RateLimit.RefreshMaxAttempts)
	assert.Equal(t, 10, cfg.RateLimit.SSOMaxAttempts)
}

func TestResolveCredentials_StaticWhenNoSecret(t *testing.T) {
	s := config.ICEServerConfig{Username: "user", Credential: "pass"}
	username, credential := s.ResolveCredentials(time.Now())
	assert.Equal(t, "user", username)
	assert.Equal(t, "pass", credential)
}

func TestResolveCredentials_GeneratesRESTCredentials(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	s := config.ICEServerConfig{Secret: "topsecret", CredentialTTL: 3600}

	username, credential := s.ResolveCredentials(now)

	// Username is the expiry unix timestamp (now + ttl).
	require.Equal(t, "1003600", username)

	// Credential is base64(HMAC-SHA1(secret, username)) — computed independently.
	mac := hmac.New(sha1.New, []byte("topsecret"))
	mac.Write([]byte(username))
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	assert.Equal(t, want, credential)
}

func TestResolveCredentials_SecretTakesPriorityAndPrefixesUsername(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	s := config.ICEServerConfig{Username: "alice", Credential: "static", Secret: "topsecret", CredentialTTL: 3600}

	username, credential := s.ResolveCredentials(now)

	require.Equal(t, "1003600:alice", username)
	assert.NotEqual(t, "static", credential)
}

func TestResolveCredentials_DefaultsTTLWhenUnset(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	s := config.ICEServerConfig{Secret: "topsecret"}
	username, _ := s.ResolveCredentials(now)
	assert.Equal(t, "1086400", username) // now + default 86400s
}
