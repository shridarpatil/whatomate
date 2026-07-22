package audit_test

import (
	"testing"

	"github.com/shridarpatil/whatomate/internal/audit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeChanges_CreateRecordsAllFields(t *testing.T) {
	type Foo struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	changes := audit.ComputeChanges(nil, Foo{Name: "x", Count: 3})
	require.Len(t, changes, 2)

	got := make(map[string]map[string]any, len(changes))
	for _, c := range changes {
		got[c["field"].(string)] = c
	}
	assert.Nil(t, got["name"]["old_value"])
	assert.Equal(t, "x", got["name"]["new_value"])
	assert.Nil(t, got["count"]["old_value"])
	assert.Equal(t, float64(3), got["count"]["new_value"])
}

func TestComputeChanges_DeleteRecordsAllFields(t *testing.T) {
	type Foo struct {
		Name string `json:"name"`
	}
	changes := audit.ComputeChanges(Foo{Name: "x"}, nil)
	require.Len(t, changes, 1)
	assert.Equal(t, "name", changes[0]["field"])
	assert.Equal(t, "x", changes[0]["old_value"])
	assert.Nil(t, changes[0]["new_value"])
}

func TestComputeChanges_UpdateOnlyDifferingFields(t *testing.T) {
	type Foo struct {
		A string `json:"a"`
		B string `json:"b"`
	}
	changes := audit.ComputeChanges(Foo{A: "1", B: "2"}, Foo{A: "1", B: "20"})
	assert.Len(t, changes, 1, "only the changed field should be in the diff")
	assert.Equal(t, "b", changes[0]["field"])
	assert.Equal(t, "2", changes[0]["old_value"])
	assert.Equal(t, "20", changes[0]["new_value"])
}

func TestComputeChanges_SkipsMetadataFields(t *testing.T) {
	type Foo struct {
		ID             string `json:"id"`
		OrganizationID string `json:"organization_id"`
		CreatedAt      string `json:"created_at"`
		UpdatedByID    string `json:"updated_by_id"`
		Visible        string `json:"visible"`
	}
	changes := audit.ComputeChanges(nil, Foo{
		ID: "abc", OrganizationID: "org", CreatedAt: "now", UpdatedByID: "u", Visible: "yes",
	})
	got := make(map[string]bool, len(changes))
	for _, c := range changes {
		got[c["field"].(string)] = true
	}
	assert.False(t, got["id"], "id is in skipFields")
	assert.False(t, got["organization_id"])
	assert.False(t, got["created_at"])
	assert.False(t, got["updated_by_id"])
	assert.True(t, got["visible"], "non-skip fields must be reported")
}

func TestComputeChanges_FlattenedJSONBField(t *testing.T) {
	// "response_content" is JSONB; only its "body" sub-key should be diffed.
	type Foo struct {
		ResponseContent map[string]any `json:"response_content"`
	}
	old := Foo{ResponseContent: map[string]any{"body": "hi", "buttons": []string{"a"}}}
	updated := Foo{ResponseContent: map[string]any{"body": "hello", "buttons": []string{"a"}}}

	changes := audit.ComputeChanges(old, updated)
	require.Len(t, changes, 1)
	assert.Equal(t, "response_content", changes[0]["field"])
	assert.Equal(t, "hi", changes[0]["old_value"], "old should be the body sub-field, not the whole map")
	assert.Equal(t, "hello", changes[0]["new_value"])
}

func TestComputeChanges_FlattenedField_NoDiff(t *testing.T) {
	// Same body, different non-flattened sub-key → no diff.
	type Foo struct {
		ResponseContent map[string]any `json:"response_content"`
	}
	old := Foo{ResponseContent: map[string]any{"body": "hi", "buttons": []string{"a"}}}
	updated := Foo{ResponseContent: map[string]any{"body": "hi", "buttons": []string{"a", "b"}}}

	changes := audit.ComputeChanges(old, updated)
	assert.Empty(t, changes, "only the body sub-field is tracked; button changes are ignored")
}

func TestFormatFieldLabel(t *testing.T) {
	cases := map[string]string{
		"":                      "",
		"name":                  "Name",
		"phone_number":          "Phone Number",
		"chatbot_reminder_sent": "Chatbot Reminder Sent",
	}
	for in, want := range cases {
		assert.Equal(t, want, audit.FormatFieldLabel(in), "input=%q", in)
	}
}
