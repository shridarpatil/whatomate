package handlers

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateOrgLogo(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	jpeg := []byte("\xff\xd8\xff\xe0\x00\x10JFIF")
	b64 := base64.StdEncoding.EncodeToString

	tests := []struct {
		name string
		logo string
		want bool
	}{
		{"png", "data:image/png;base64," + b64(png), true},
		{"jpeg", "data:image/jpeg;base64," + b64(jpeg), true},
		{"declared type must match bytes", "data:image/png;base64," + b64(jpeg), false},
		{"svg rejected", "data:image/svg+xml;base64," + b64([]byte("<svg onload=alert(1)>")), false},
		{"not base64", "data:image/png;base64,***", false},
		{"missing base64 marker", "data:image/png," + b64(png), false},
		{"not a data url", "https://example.com/logo.png", false},
		{"empty payload", "data:image/png;base64,", false},
		{"too large", "data:image/png;base64," + b64(append(png, make([]byte, maxOrgLogoBytes)...)), false},
		{"oversized payload rejected before decoding", "data:image/png;base64," + strings.Repeat("A", 200000), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, validateOrgLogo(tt.logo))
		})
	}
}
