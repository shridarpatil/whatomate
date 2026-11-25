package handlers

import (
	"testing"

	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestSSOButtonName(t *testing.T) {
	assert.Equal(t, "Acme Okta", ssoButtonName(&models.SSOProvider{Provider: "custom", DisplayName: "Acme Okta"}))
	assert.Equal(t, "Custom SSO", ssoButtonName(&models.SSOProvider{Provider: "custom"}))
	assert.Equal(t, "GitHub", ssoButtonName(&models.SSOProvider{Provider: "github"}))
	assert.Equal(t, "Work Google", ssoButtonName(&models.SSOProvider{Provider: "google", DisplayName: "Work Google"}))
}
