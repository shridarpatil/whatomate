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
			name:     "nil JSONB returns nil",
			input:    nil,
			wantNil:  true,
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
				"outer": map[string]interface{}{
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
		input   interface{}
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
		input   interface{}
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
				map[string]interface{}{"id": "1", "title": "Button 1"},
				map[string]interface{}{"id": "2", "title": "Button 2"},
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

func TestJSONBArray_Scan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   interface{}
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
