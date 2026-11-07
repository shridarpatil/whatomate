package contactutil

import "strings"

// defaultCountryCode is prefixed to bare national numbers. It matches the rule
// already used by RSVP capture/import and reminders: a bare 10-digit number is
// an Indian mobile and WhatsApp reports it as 91XXXXXXXXXX.
const defaultCountryCode = "91"

const nationalLength = 10

// CanonicalPhone reduces a phone number to the single form contacts are stored
// under: digits only, with country code, no "+". It is the same form WhatsApp
// uses for wa_id, so incoming messages and manually added contacts meet on one
// row.
//
//	+91 98765 43210 -> 919876543210
//	919876543210    -> 919876543210
//	9876543210      -> 919876543210
//	09876543210     -> 919876543210
//	00919876543210  -> 919876543210 (00 international prefix dropped)
//
// Input with no digits at all is returned trimmed and without a leading "+",
// so non-phone identifiers keep the previous behaviour.
func CanonicalPhone(raw string) string {
	trimmed := strings.TrimSpace(raw)
	digits := digitsOnly(trimmed)
	if digits == "" {
		return strings.TrimPrefix(trimmed, "+")
	}
	if strings.HasPrefix(digits, "00") {
		digits = digits[2:]
	}
	switch {
	case len(digits) == nationalLength:
		return defaultCountryCode + digits
	case len(digits) == nationalLength+1 && digits[0] == '0':
		return defaultCountryCode + digits[1:]
	}
	return digits
}

// PhoneVariants lists the stored forms that refer to the same number as raw,
// canonical form first. It covers rows written before canonicalisation
// (with "+", bare 10-digit, leading 0) so lookups still find them.
func PhoneVariants(raw string) []string {
	canonical := CanonicalPhone(raw)
	seen := map[string]bool{}
	out := make([]string, 0, 8)
	add := func(v string) {
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	add(canonical)
	add("+" + canonical)
	if national, ok := nationalPart(canonical); ok {
		add(national)
		add("0" + national)
		add("+" + national)
	}
	add(strings.TrimSpace(raw))
	return out
}

// nationalPart returns the 10-digit national number of a canonical
// default-country number.
func nationalPart(canonical string) (string, bool) {
	if len(canonical) == len(defaultCountryCode)+nationalLength &&
		strings.HasPrefix(canonical, defaultCountryCode) {
		return canonical[len(defaultCountryCode):], true
	}
	return "", false
}

// IsMergeablePhone reports whether a canonical value looks like a real phone
// number (10-15 digits). Duplicate merging ignores anything else.
func IsMergeablePhone(canonical string) bool {
	if len(canonical) < 10 || len(canonical) > 15 {
		return false
	}
	return digitsOnly(canonical) == canonical
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
