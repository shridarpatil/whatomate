package handlers

import (
	"testing"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestMessageMediaURL(t *testing.T) {
	msgID := uuid.New()

	tests := []struct {
		name     string
		msg      models.Message
		expected string
	}{
		{
			name:     "no media",
			msg:      models.Message{BaseModel: models.BaseModel{ID: msgID}},
			expected: "",
		},
		{
			name:     "media stored",
			msg:      models.Message{BaseModel: models.BaseModel{ID: msgID}, MediaURL: "images/test-image.jpg"},
			expected: "/api/media/" + msgID.String(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, messageMediaURL(&tt.msg))
		})
	}
}
