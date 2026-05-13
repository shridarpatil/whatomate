package database_test

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseTestRedisURL splits TEST_REDIS_URL (redis://[user:pass@]host:port[/db])
// into a RedisConfig so we can pass individual fields to NewRedis.
func parseTestRedisConfig(t *testing.T) *config.RedisConfig {
	t.Helper()

	raw := os.Getenv("TEST_REDIS_URL")
	if raw == "" {
		t.Skip("TEST_REDIS_URL not set, skipping Redis test")
	}

	// Strip scheme
	raw = strings.TrimPrefix(raw, "redis://")

	cfg := &config.RedisConfig{
		Port: 6379,
	}

	// Optional userinfo
	if at := strings.LastIndex(raw, "@"); at != -1 {
		userinfo := raw[:at]
		raw = raw[at+1:]
		if colon := strings.Index(userinfo, ":"); colon != -1 {
			cfg.Username = userinfo[:colon]
			cfg.Password = userinfo[colon+1:]
		} else {
			cfg.Username = userinfo
		}
	}

	// Optional /db suffix
	if slash := strings.Index(raw, "/"); slash != -1 {
		dbStr := raw[slash+1:]
		raw = raw[:slash]
		if db, err := strconv.Atoi(dbStr); err == nil {
			cfg.DB = db
		}
	}

	// host:port
	if colon := strings.LastIndex(raw, ":"); colon != -1 {
		cfg.Host = raw[:colon]
		if port, err := strconv.Atoi(raw[colon+1:]); err == nil {
			cfg.Port = port
		}
	} else {
		cfg.Host = raw
	}

	return cfg
}

func TestNewRedis_ConnectsWithDefaultOptions(t *testing.T) {
	cfg := parseTestRedisConfig(t)

	client, err := database.NewRedis(cfg)
	require.NoError(t, err, "NewRedis should connect successfully")
	require.NotNil(t, client)
	defer client.Close() //nolint:errcheck
}

func TestNewRedis_EmptyUsernameUsesDefaultUser(t *testing.T) {
	cfg := parseTestRedisConfig(t)
	cfg.Username = "" // explicit zero value — should fall back to Redis "default" user

	client, err := database.NewRedis(cfg)
	require.NoError(t, err, "empty username should connect using the default ACL user")
	require.NotNil(t, client)
	defer client.Close() //nolint:errcheck
}

func TestNewRedis_TLSFalseDoesNotSetTLSConfig(t *testing.T) {
	cfg := parseTestRedisConfig(t)
	cfg.TLS = false

	client, err := database.NewRedis(cfg)
	require.NoError(t, err)
	require.NotNil(t, client)
	defer client.Close() //nolint:errcheck

	// Connection must still be usable (PING succeeds)
	ctx := t.Context()
	assert.NoError(t, client.Ping(ctx).Err())
}

func TestNewRedis_InvalidHostReturnsError(t *testing.T) {
	parseTestRedisConfig(t) // still skip if TEST_REDIS_URL is unset

	cfg := &config.RedisConfig{
		Host: "localhost",
		Port: 19999, // nothing listening here
	}

	client, err := database.NewRedis(cfg)
	assert.Error(t, err, "should fail when Redis is unreachable")
	assert.Nil(t, client)
}

func TestNewRedis_TLSEnabledFailsAgainstPlainTextServer(t *testing.T) {
	cfg := parseTestRedisConfig(t)
	cfg.TLS = true // our test Redis has no TLS — handshake must fail

	client, err := database.NewRedis(cfg)
	assert.Error(t, err, "TLS handshake against a plain-text server should fail")
	assert.Nil(t, client)
}
