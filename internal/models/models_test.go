package models_test

import (
	"testing"

	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONB_Value(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    models.JSONB
		wantJSON string
		wantNil  bool
	}{
		{
			name:    "nil JSONB returns nil",
			input:   nil,
			wantNil: true,
		},
		{
			name:     "empty JSONB returns empty object",
			input:    models.JSONB{},
			wantJSON: "{}",
		},
		{
			name: "JSONB with values",
			input: models.JSONB{
				"key1": "value1",
				"key2": 123,
				"key3": true,
			},
			wantJSON: `{"key1":"value1","key2":123,"key3":true}`,
		},
		{
			name: "nested JSONB",
			input: models.JSONB{
				"outer": map[string]any{
					"inner": "value",
				},
			},
			wantJSON: `{"outer":{"inner":"value"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			val, err := tt.input.Value()
			require.NoError(t, err)

			if tt.wantNil {
				assert.Nil(t, val)
				return
			}

			// Value returns []byte from json.Marshal
			bytes, ok := val.([]byte)
			require.True(t, ok, "expected []byte, got %T", val)
			assert.JSONEq(t, tt.wantJSON, string(bytes))
		})
	}
}

func TestJSONB_Scan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   any
		want    models.JSONB
		wantErr bool
	}{
		{
			name:  "nil input",
			input: nil,
			want:  nil,
		},
		{
			name:  "empty object bytes",
			input: []byte("{}"),
			want:  models.JSONB{},
		},
		{
			name:  "object with values",
			input: []byte(`{"key":"value","num":42}`),
			want: models.JSONB{
				"key": "value",
				"num": float64(42), // JSON numbers decode as float64
			},
		},
		{
			name:    "invalid type",
			input:   "not bytes",
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			input:   []byte("not json"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var j models.JSONB
			err := j.Scan(tt.input)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, j)
		})
	}
}

func TestStringArray_Value(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    models.StringArray
		wantJSON string
		wantNil  bool
	}{
		{
			name:    "nil StringArray returns nil",
			input:   nil,
			wantNil: true,
		},
		{
			name:     "empty StringArray returns empty array",
			input:    models.StringArray{},
			wantJSON: "[]",
		},
		{
			name:     "StringArray with values",
			input:    models.StringArray{"a", "b", "c"},
			wantJSON: `["a","b","c"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			val, err := tt.input.Value()
			require.NoError(t, err)

			if tt.wantNil {
				assert.Nil(t, val)
				return
			}

			bytes, ok := val.([]byte)
			require.True(t, ok, "expected []byte, got %T", val)
			assert.JSONEq(t, tt.wantJSON, string(bytes))
		})
	}
}

func TestStringArray_Scan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   any
		want    models.StringArray
		wantErr bool
	}{
		{
			name:  "nil input",
			input: nil,
			want:  nil,
		},
		{
			name:  "empty array bytes",
			input: []byte("[]"),
			want:  models.StringArray{},
		},
		{
			name:  "array with values",
			input: []byte(`["one","two","three"]`),
			want:  models.StringArray{"one", "two", "three"},
		},
		{
			name:    "invalid type",
			input:   123,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var s models.StringArray
			err := s.Scan(tt.input)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, s)
		})
	}
}

func TestJSONBArray_Value(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    models.JSONBArray
		wantJSON string
		wantNil  bool
	}{
		{
			name:    "nil JSONBArray returns nil",
			input:   nil,
			wantNil: true,
		},
		{
			name:     "empty JSONBArray returns empty array",
			input:    models.JSONBArray{},
			wantJSON: "[]",
		},
		{
			name: "JSONBArray with values",
			input: models.JSONBArray{
				map[string]any{"id": "1", "title": "Button 1"},
				map[string]any{"id": "2", "title": "Button 2"},
			},
			wantJSON: `[{"id":"1","title":"Button 1"},{"id":"2","title":"Button 2"}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			val, err := tt.input.Value()
			require.NoError(t, err)

			if tt.wantNil {
				assert.Nil(t, val)
				return
			}

			bytes, ok := val.([]byte)
			require.True(t, ok, "expected []byte, got %T", val)
			assert.JSONEq(t, tt.wantJSON, string(bytes))
		})
	}
}

func TestConversationsViewAllPermission(t *testing.T) {
	// The permission must exist in the default set.
	found := false
	for _, p := range models.DefaultPermissions() {
		if p.Resource == models.ResourceConversations && p.Action == models.ActionViewAll {
			found = true
		}
	}
	assert.True(t, found, "conversations:view_all must be a default permission")

	roles := models.SystemRolePermissions()
	// admin gets every default permission automatically.
	assert.Contains(t, roles["admin"], "conversations:view_all")
	// manager is a supervisor and must see all conversations.
	assert.Contains(t, roles["manager"], "conversations:view_all")
	// agent must NOT — that is the whole point.
	assert.NotContains(t, roles["agent"], "conversations:view_all")
}

// TestOccurrenceStagesPermissionNotForAgent guards the pipeline-admin
// permissions: managing stages is a manager/admin concern, not something
// every agent gets alongside occurrences:read/write.
func TestOccurrenceStagesPermissionNotForAgent(t *testing.T) {
	roles := models.SystemRolePermissions()
	// manager administers the pipeline and must have all three.
	assert.Contains(t, roles["manager"], "occurrences.stages:read")
	assert.Contains(t, roles["manager"], "occurrences.stages:write")
	assert.Contains(t, roles["manager"], "occurrences.stages:delete")
	// agent must NOT — that is the whole point.
	assert.NotContains(t, roles["agent"], "occurrences.stages:read")
	assert.NotContains(t, roles["agent"], "occurrences.stages:write")
	assert.NotContains(t, roles["agent"], "occurrences.stages:delete")
}

// TestAgentHasContactNamePermission guards the seed data behind the
// contact-rename feature: SeedSystemRolesForOrg and FixSystemRolePermissions
// grant from this list, and the backfill's organisation guard skips any org
// that already holds the permission — so a fresh organisation only ever gets
// contacts.name:write through this entry. Drop it and every new org's agents
// get 403 on rename, silently, with the rest of the suite still green.
func TestAgentHasContactNamePermission(t *testing.T) {
	roles := models.SystemRolePermissions()
	assert.Contains(t, roles["agent"], "contacts.name:write")
	// The permission's whole point is narrower than contacts:write — an
	// agent who can rename must still not be able to edit the rest of the
	// contact. Without this, "fixing" the feature by widening the agent to
	// contacts:write would still pass.
	assert.NotContains(t, roles["agent"], "contacts:write")
}

func TestViewTeamPermissionInCatalogButNotDefaultRoles(t *testing.T) {
	// It must exist in the catalog so admins can assign it.
	found := false
	for _, p := range models.DefaultPermissions() {
		if p.Resource == models.ResourceConversations && p.Action == models.ActionViewTeam {
			found = true
		}
	}
	assert.True(t, found, "conversations:view_team must be in DefaultPermissions")

	// It must NOT be granted to manager or agent by default.
	roles := models.SystemRolePermissions()
	assert.NotContains(t, roles["manager"], "conversations:view_team")
	assert.NotContains(t, roles["agent"], "conversations:view_team")
}

func TestJSONBArray_Scan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   any
		wantLen int
		wantErr bool
	}{
		{
			name:    "nil input",
			input:   nil,
			wantLen: 0,
		},
		{
			name:    "empty array bytes",
			input:   []byte("[]"),
			wantLen: 0,
		},
		{
			name:    "array with objects",
			input:   []byte(`[{"id":"1"},{"id":"2"}]`),
			wantLen: 2,
		},
		{
			name:    "invalid type",
			input:   "not bytes",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var j models.JSONBArray
			err := j.Scan(tt.input)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tt.wantLen == 0 && tt.input == nil {
				assert.Nil(t, j)
			} else {
				assert.Len(t, j, tt.wantLen)
			}
		})
	}
}
