package config_test

import (
	"testing"

	"github.com/nikyjain/whatomate/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_MCPEnabledByDefault(t *testing.T) {
	cfg, err := config.Load(writeConfig(t, ""))
	require.NoError(t, err)
	assert.True(t, cfg.MCP.Enabled)
}

func TestLoad_MCPCanBeDisabledInFile(t *testing.T) {
	cfg, err := config.Load(writeConfig(t, "[mcp]\nenabled = false\n"))
	require.NoError(t, err)
	assert.False(t, cfg.MCP.Enabled)
}

func TestLoad_MCPCanBeDisabledByEnv(t *testing.T) {
	t.Setenv("WHATOMATE_MCP__ENABLED", "false")
	cfg, err := config.Load(writeConfig(t, ""))
	require.NoError(t, err)
	assert.False(t, cfg.MCP.Enabled)
}

func TestLoad_MCPLegacyEnvStillWorks(t *testing.T) {
	t.Setenv("WHATOMATE_MCP_ENABLED", "false")
	cfg, err := config.Load(writeConfig(t, ""))
	require.NoError(t, err)
	assert.False(t, cfg.MCP.Enabled)
}
