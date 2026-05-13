package whatsapp

// Recipient identifies a WhatsApp user by phone number and/or BSUID.
// Meta accepts both: phone number via "to" and BSUID via "recipient".
// When both are provided, phone number takes precedence.
type Recipient struct {
	Phone string // Phone number (e.g., "16505551234")
	BSUID string // Business-Scoped User ID (e.g., "US.13491208655302741918")
}

// SetOnPayload sets the "to" and/or "recipient" fields on a message payload.
func (r Recipient) SetOnPayload(payload map[string]any) {
	if r.Phone != "" {
		payload["to"] = r.Phone
	}
	if r.BSUID != "" {
		payload["recipient"] = r.BSUID
	}
}
