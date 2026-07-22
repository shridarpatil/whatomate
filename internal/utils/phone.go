package utils

// MaskPhoneNumber masks a phone number showing only last 4 digits.
func MaskPhoneNumber(phone string) string {
	if len(phone) <= 4 {
		return phone
	}
	masked := ""
	for i := 0; i < len(phone)-4; i++ {
		masked += "*"
	}
	return masked + phone[len(phone)-4:]
}

// LooksLikePhoneNumber checks if a string looks like a phone number
// (mostly digits, optionally with common phone formatting characters).
func LooksLikePhoneNumber(s string) bool {
	if len(s) < 7 {
		return false
	}
	digitCount := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			digitCount++
		}
	}
	return digitCount >= 7 && float64(digitCount)/float64(len(s)) > 0.7
}

// MaskIfPhoneNumber masks a string if it looks like a phone number.
func MaskIfPhoneNumber(s string) string {
	if LooksLikePhoneNumber(s) {
		return MaskPhoneNumber(s)
	}
	return s
}
