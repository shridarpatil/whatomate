package contactutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCanonicalPhone(t *testing.T) {
	cases := map[string]string{
		"919876543210":      "919876543210",
		"+919876543210":     "919876543210",
		"+91 98765 43210":   "919876543210",
		"9876543210":        "919876543210",
		"09876543210":       "919876543210",
		"00919876543210":    "919876543210",
		" 98765-43210 ":     "919876543210",
		"+14155552671":      "14155552671",
		"":                  "",
		"+not-a-phone":      "not-a-phone",
		"971501234567":      "971501234567",
	}
	for in, want := range cases {
		assert.Equal(t, want, CanonicalPhone(in), "input %q", in)
	}
}

func TestPhoneVariants(t *testing.T) {
	v := PhoneVariants("9876543210")
	assert.Equal(t, "919876543210", v[0], "canonical form first")
	for _, want := range []string{"+919876543210", "9876543210", "09876543210", "+9876543210"} {
		assert.Contains(t, v, want)
	}
}

func TestLooksLikePhone(t *testing.T) {
	assert.True(t, looksLikePhone("+91 (98765) 43210"))
	assert.False(t, looksLikePhone("merged:1234"))
	assert.False(t, looksLikePhone(""))
}
