package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/crypto"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cacheTestApp builds a minimal App configured for cache tests, with both DB
// and Redis available. Skips the test if Redis isn't reachable.
func cacheTestApp(t *testing.T) *App {
	t.Helper()
	db := testutil.SetupTestDB(t)
	rdb := testutil.SetupTestRedis(t)
	if rdb == nil {
		t.Skip("TEST_REDIS_URL not set")
	}
	return &App{
		DB:    db,
		Log:   testutil.NopLogger(),
		Redis: rdb,
		Config: &config.Config{
			JWT: config.JWTConfig{Secret: testutil.TestJWTSecret, AccessExpiryMins: 15, RefreshExpiryDays: 7},
		},
	}
}

// makeAccount inserts a WhatsApp account with the given phone ID and tokens.
func makeAccount(t *testing.T, app *App, orgID uuid.UUID, phoneID, accessToken, appSecret string) *models.WhatsAppAccount {
	t.Helper()
	acc := &models.WhatsAppAccount{
		BaseModel:          models.BaseModel{ID: uuid.New()},
		OrganizationID:     orgID,
		Name:               "cache-acc-" + uuid.New().String()[:8],
		PhoneID:            phoneID,
		BusinessID:         "biz-" + uuid.New().String()[:8],
		AccessToken:        accessToken,
		AppSecret:          appSecret,
		WebhookVerifyToken: "vt-" + uuid.New().String()[:8],
		APIVersion:         "v18.0",
		Status:             "active",
	}
	require.NoError(t, app.DB.Create(acc).Error)
	return acc
}

// --- getWhatsAppAccountCached ---

func TestGetWhatsAppAccountCached_CacheMissPopulatesCache(t *testing.T) {
	app := cacheTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	phoneID := "phone-" + uuid.New().String()[:8]
	acc := makeAccount(t, app, org.ID, phoneID, "tok", "secret")

	// Ensure cache is clean.
	cacheKey := fmt.Sprintf("%s%s", whatsappAccountCachePrefix, phoneID)
	app.Redis.Del(context.Background(), cacheKey)

	got, err := app.getWhatsAppAccountCached(phoneID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, acc.ID, got.ID)
	assert.Equal(t, "tok", got.AccessToken, "DB → cache path must return decrypted access token")
	assert.Equal(t, "secret", got.AppSecret)

	// Cache should now be populated.
	cached, err := app.Redis.Get(context.Background(), cacheKey).Result()
	require.NoError(t, err)
	require.NotEmpty(t, cached)
}

func TestGetWhatsAppAccountCached_CacheHitSkipsDB(t *testing.T) {
	app := cacheTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	phoneID := "phone-" + uuid.New().String()[:8]
	acc := makeAccount(t, app, org.ID, phoneID, "real-tok", "real-secret")

	// Plant a fake cache entry with a different name → if the function reads from
	// cache, we'll see "from-cache"; if it falls through to DB we'll see acc.Name.
	cacheData := whatsAppAccountCache{
		WhatsAppAccount: models.WhatsAppAccount{
			BaseModel:      models.BaseModel{ID: acc.ID},
			OrganizationID: org.ID,
			Name:           "from-cache",
			PhoneID:        phoneID,
		},
		AccessToken: "cached-tok",
		AppSecret:   "cached-secret",
	}
	data, err := json.Marshal(cacheData)
	require.NoError(t, err)
	cacheKey := fmt.Sprintf("%s%s", whatsappAccountCachePrefix, phoneID)
	require.NoError(t, app.Redis.Set(context.Background(), cacheKey, data, time.Hour).Err())

	got, err := app.getWhatsAppAccountCached(phoneID)
	require.NoError(t, err)
	assert.Equal(t, "from-cache", got.Name, "cache hit must short-circuit the DB read")
	assert.Equal(t, "cached-tok", got.AccessToken,
		"cached AccessToken must be restored on the WhatsAppAccount even though it has json:\"-\"")
	assert.Equal(t, "cached-secret", got.AppSecret,
		"cached AppSecret must be restored")
}

func TestGetWhatsAppAccountCached_NotFoundReturnsError(t *testing.T) {
	app := cacheTestApp(t)
	_, err := app.getWhatsAppAccountCached("phone-does-not-exist-" + uuid.New().String()[:8])
	require.Error(t, err)
}

func TestInvalidateWhatsAppAccountCache_DeletesKey(t *testing.T) {
	app := cacheTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	phoneID := "phone-" + uuid.New().String()[:8]
	makeAccount(t, app, org.ID, phoneID, "tok", "secret")

	// Populate the cache.
	_, err := app.getWhatsAppAccountCached(phoneID)
	require.NoError(t, err)

	cacheKey := fmt.Sprintf("%s%s", whatsappAccountCachePrefix, phoneID)
	exists, err := app.Redis.Exists(context.Background(), cacheKey).Result()
	require.NoError(t, err)
	require.Equal(t, int64(1), exists)

	app.InvalidateWhatsAppAccountCache(phoneID)

	exists, err = app.Redis.Exists(context.Background(), cacheKey).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), exists, "invalidation must remove the cache key")
}

// --- decryptAccountSecrets handles encrypted + legacy plaintext ---

func TestDecryptAccountSecrets_DecryptsEncryptedValues(t *testing.T) {
	app := cacheTestApp(t)
	app.Config.App.EncryptionKey = "this-is-a-32-character-test-key-XX"

	encTok, err := crypto.Encrypt("plain-token", app.Config.App.EncryptionKey)
	require.NoError(t, err)
	encSecret, err := crypto.Encrypt("plain-secret", app.Config.App.EncryptionKey)
	require.NoError(t, err)

	acc := &models.WhatsAppAccount{AccessToken: encTok, AppSecret: encSecret}
	app.decryptAccountSecrets(acc)
	assert.Equal(t, "plain-token", acc.AccessToken)
	assert.Equal(t, "plain-secret", acc.AppSecret)
}

func TestDecryptAccountSecrets_LeavesLegacyPlaintextUnchanged(t *testing.T) {
	app := cacheTestApp(t)
	app.Config.App.EncryptionKey = "this-is-a-32-character-test-key-XX"

	// No "enc:" prefix → legacy value, returned as-is.
	acc := &models.WhatsAppAccount{AccessToken: "legacy-plain", AppSecret: "legacy-secret"}
	app.decryptAccountSecrets(acc)
	assert.Equal(t, "legacy-plain", acc.AccessToken)
	assert.Equal(t, "legacy-secret", acc.AppSecret)
}

// --- getWebhooksCached ---

func TestGetWebhooksCached_OnlyActiveAndOrgScoped(t *testing.T) {
	app := cacheTestApp(t)
	orgA := testutil.CreateTestOrganization(t, app.DB)
	orgB := testutil.CreateTestOrganization(t, app.DB)

	// Active webhook in target org.
	require.NoError(t, app.DB.Create(&models.Webhook{
		BaseModel: models.BaseModel{ID: uuid.New()}, OrganizationID: orgA.ID,
		Name: "active-A", URL: "https://a.example.com", IsActive: true,
	}).Error)
	// Inactive webhook in target org — must be excluded. GORM's default:true tag
	// makes Create ignore an explicit `false`; insert active first, then flip.
	inactive := &models.Webhook{
		BaseModel: models.BaseModel{ID: uuid.New()}, OrganizationID: orgA.ID,
		Name: "inactive-A", URL: "https://a2.example.com", IsActive: true,
	}
	require.NoError(t, app.DB.Create(inactive).Error)
	require.NoError(t, app.DB.Model(inactive).Update("is_active", false).Error)
	// Active webhook in other org — must be excluded.
	require.NoError(t, app.DB.Create(&models.Webhook{
		BaseModel: models.BaseModel{ID: uuid.New()}, OrganizationID: orgB.ID,
		Name: "active-B", URL: "https://b.example.com", IsActive: true,
	}).Error)

	// Clear cache to force DB read.
	app.InvalidateWebhooksCache(orgA.ID)

	got, err := app.getWebhooksCached(orgA.ID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "active-A", got[0].Name)
}

func TestGetWebhooksCached_CacheHitSkipsDB(t *testing.T) {
	app := cacheTestApp(t)
	orgID := uuid.New()

	// Plant a synthetic cache entry — there is no matching row in the DB.
	cacheKey := fmt.Sprintf("%s%s", webhooksCachePrefix, orgID.String())
	cached := []models.Webhook{{
		BaseModel: models.BaseModel{ID: uuid.New()}, OrganizationID: orgID,
		Name: "from-cache", IsActive: true,
	}}
	data, err := json.Marshal(cached)
	require.NoError(t, err)
	require.NoError(t, app.Redis.Set(context.Background(), cacheKey, data, time.Hour).Err())

	got, err := app.getWebhooksCached(orgID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "from-cache", got[0].Name, "cache hit must skip the DB read")
}

func TestInvalidateWebhooksCache_DeletesKey(t *testing.T) {
	app := cacheTestApp(t)
	orgID := uuid.New()
	cacheKey := fmt.Sprintf("%s%s", webhooksCachePrefix, orgID.String())
	require.NoError(t, app.Redis.Set(context.Background(), cacheKey, "[]", time.Hour).Err())

	app.InvalidateWebhooksCache(orgID)

	exists, err := app.Redis.Exists(context.Background(), cacheKey).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), exists)
}

// --- getSLAEnabledSettingsCached ---

func TestGetSLAEnabledSettingsCached_OnlySLAEnabledRows(t *testing.T) {
	app := cacheTestApp(t)
	orgA := testutil.CreateTestOrganization(t, app.DB)
	orgB := testutil.CreateTestOrganization(t, app.DB)

	// Wipe any pre-existing rows so we can assert exact counts.
	require.NoError(t, app.DB.Exec("DELETE FROM chatbot_settings").Error)
	app.InvalidateSLASettingsCache()

	require.NoError(t, app.DB.Create(&models.ChatbotSettings{
		BaseModel: models.BaseModel{ID: uuid.New()}, OrganizationID: orgA.ID,
		WhatsAppAccount: "acc-A", IsEnabled: true,
		SLA: models.SLAConfig{Enabled: true},
	}).Error)
	require.NoError(t, app.DB.Create(&models.ChatbotSettings{
		BaseModel: models.BaseModel{ID: uuid.New()}, OrganizationID: orgB.ID,
		WhatsAppAccount: "acc-B", IsEnabled: true,
		SLA: models.SLAConfig{Enabled: true},
	}).Error)
	require.NoError(t, app.DB.Create(&models.ChatbotSettings{
		BaseModel: models.BaseModel{ID: uuid.New()}, OrganizationID: orgA.ID,
		WhatsAppAccount: "acc-A2", IsEnabled: true,
		SLA: models.SLAConfig{Enabled: false},
	}).Error)

	got, err := app.getSLAEnabledSettingsCached()
	require.NoError(t, err)
	require.Len(t, got, 2, "only SLA-enabled rows must be returned, regardless of org")
	for _, s := range got {
		assert.True(t, s.SLA.Enabled)
	}
}

func TestInvalidateSLASettingsCache_DeletesKey(t *testing.T) {
	app := cacheTestApp(t)
	require.NoError(t, app.Redis.Set(context.Background(), slaSettingsCacheKey, "[]", time.Hour).Err())

	app.InvalidateSLASettingsCache()

	exists, err := app.Redis.Exists(context.Background(), slaSettingsCacheKey).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), exists)
}

// --- deleteKeysByPattern ---

func TestDeleteKeysByPattern_RemovesAllMatchingKeys(t *testing.T) {
	app := cacheTestApp(t)
	ctx := context.Background()

	prefix := "test:cache:" + uuid.New().String()[:8] + ":"
	for i := range 5 {
		require.NoError(t, app.Redis.Set(ctx, fmt.Sprintf("%s%d", prefix, i), "x", time.Hour).Err())
	}
	// Sentinel key with a similar but non-matching prefix.
	other := "test:other:" + uuid.New().String()[:8]
	require.NoError(t, app.Redis.Set(ctx, other, "y", time.Hour).Err())

	app.deleteKeysByPattern(ctx, prefix+"*")

	for i := range 5 {
		exists, err := app.Redis.Exists(ctx, fmt.Sprintf("%s%d", prefix, i)).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(0), exists, "key %d should have been deleted", i)
	}
	exists, err := app.Redis.Exists(ctx, other).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), exists, "non-matching key must be left alone")
}

// --- InvalidateChatbotFlowsCache (uses Del on a single key) ---

func TestInvalidateChatbotFlowsCache_DeletesKey(t *testing.T) {
	app := cacheTestApp(t)
	orgID := uuid.New()
	cacheKey := fmt.Sprintf("%s%s", flowsCachePrefix, orgID.String())
	require.NoError(t, app.Redis.Set(context.Background(), cacheKey, "[]", time.Hour).Err())

	app.InvalidateChatbotFlowsCache(orgID)

	exists, err := app.Redis.Exists(context.Background(), cacheKey).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), exists)
}

// --- InvalidateChatbotSettingsCache (pattern-based) ---

func TestInvalidateChatbotSettingsCache_DeletesAllAccountVariants(t *testing.T) {
	app := cacheTestApp(t)
	ctx := context.Background()
	orgID := uuid.New()

	// Plant several account-specific cache keys for the same org.
	keyA := fmt.Sprintf("%s%s:%s", settingsCachePrefix, orgID.String(), "acc-A")
	keyB := fmt.Sprintf("%s%s:%s", settingsCachePrefix, orgID.String(), "acc-B")
	keyOther := fmt.Sprintf("%s%s:%s", settingsCachePrefix, uuid.New().String(), "acc-X")
	require.NoError(t, app.Redis.Set(ctx, keyA, "{}", time.Hour).Err())
	require.NoError(t, app.Redis.Set(ctx, keyB, "{}", time.Hour).Err())
	require.NoError(t, app.Redis.Set(ctx, keyOther, "{}", time.Hour).Err())

	app.InvalidateChatbotSettingsCache(orgID)

	// Both org keys gone…
	for _, k := range []string{keyA, keyB} {
		exists, err := app.Redis.Exists(ctx, k).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(0), exists, "expected %s to be deleted", k)
	}
	// …other org's key untouched.
	exists, err := app.Redis.Exists(ctx, keyOther).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), exists, "other org's cache must not be invalidated")
}
