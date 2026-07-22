package audit

import (
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"gorm.io/gorm"
)

// skipFields are metadata fields that should never appear in diffs
var skipFields = map[string]bool{
	"id": true, "created_at": true, "updated_at": true,
	"deleted_at": true, "organization_id": true,
	"created_by_id": true, "updated_by_id": true,
	"created_by": true, "updated_by": true,
	"organization": true, "members": true,
	"webhook_verify_token": true,
	"api_config":           true,
	"conditions":           true,
	"active_from":          true,
	"active_until":         true,
	"whatsapp_account":     true,
	"case_sensitive":       true,
	"completion_config":    true,
	"panel_config":         true,
	"canvas_layout":        true,
	"steps":                true,
	"initial_template":     true,
	"cancel_keywords":      true,
	"trigger_button_id":    true,
	"initial_message_type": true,
	"initial_template_id":  true,
	"timeout_message":      true,
	"menu":                      true,
	"welcome_audio_url":         true,
	"header_media_id":           true,
	"buttons":                   true,
	"sample_values":             true,
	"meta_template_id":          true,
	"header_media_local_path":   true,
	"template_id":               true,
	"recipients":                true,
	"template":                  true,
	"creator":                   true,
}

// flattenFields extracts readable sub-fields from JSONB objects.
// e.g. response_content: {body: "hello", buttons: [...]} becomes
// "response_message": "hello" (buttons are skipped).
var flattenFields = map[string]string{
	"response_content": "body",
}

// ComputeChanges compares old and new structs via JSON serialization.
// Pass nil for oldData on create, nil for newData on delete.
func ComputeChanges(oldData, newData any) []map[string]any {
	oldMap := toMap(oldData)
	newMap := toMap(newData)

	var changes []map[string]any

	if oldData == nil {
		for key, val := range newMap {
			if skipFields[key] {
				continue
			}
			changes = append(changes, map[string]any{
				"field": key, "old_value": nil, "new_value": val,
			})
		}
		return changes
	}

	if newData == nil {
		for key, val := range oldMap {
			if skipFields[key] {
				continue
			}
			changes = append(changes, map[string]any{
				"field": key, "old_value": val, "new_value": nil,
			})
		}
		return changes
	}

	for key, newVal := range newMap {
		if skipFields[key] {
			continue
		}
		oldVal := oldMap[key]
		// Flatten JSONB fields: extract a specific sub-key as a readable field
		if subKey, ok := flattenFields[key]; ok {
			oldSub := extractSubField(oldVal, subKey)
			newSub := extractSubField(newVal, subKey)
			if !jsonEqual(oldSub, newSub) {
				changes = append(changes, map[string]any{
					"field": key, "old_value": oldSub, "new_value": newSub,
				})
			}
			continue
		}
		if !jsonEqual(oldVal, newVal) {
			changes = append(changes, map[string]any{
				"field": key, "old_value": oldVal, "new_value": newVal,
			})
		}
	}
	return changes
}

// LogAudit creates an audit log entry asynchronously.
// Optional extraChanges are appended to the computed diff (useful for masked sensitive fields).
func LogAudit(
	db *gorm.DB,
	orgID, userID uuid.UUID,
	userName string,
	resourceType string,
	resourceID uuid.UUID,
	action models.AuditAction,
	oldData, newData any,
	extraChanges ...map[string]any,
) {
	changes := ComputeChanges(oldData, newData)
	changes = append(changes, extraChanges...)

	if action == models.AuditActionUpdated && len(changes) == 0 {
		return
	}

	changesArr := make(models.JSONBArray, len(changes))
	for i, c := range changes {
		changesArr[i] = c
	}

	entry := models.AuditLog{
		OrganizationID: orgID,
		ResourceType:   resourceType,
		ResourceID:     resourceID,
		UserID:         userID,
		UserName:       userName,
		Action:         action,
		Changes:        changesArr,
	}

	go func() {
		if err := db.Create(&entry).Error; err != nil {
			slog.Error("failed to create audit log", "error", err)
		}
	}()
}

func extractSubField(val any, key string) any {
	if m, ok := val.(map[string]any); ok {
		return m[key]
	}
	return nil
}

// GetUserName fetches a user's full name for audit logging.
func GetUserName(db *gorm.DB, userID uuid.UUID) string {
	var name string
	db.Model(&models.User{}).Where("id = ?", userID).Pluck("full_name", &name)
	if name == "" {
		return "Unknown"
	}
	return name
}

func toMap(v any) map[string]any {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	return m
}

func jsonEqual(a, b any) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return string(aj) == string(bj)
}

// FormatFieldLabel converts snake_case field names to human-readable labels.
func FormatFieldLabel(field string) string {
	// Simple conversion: replace underscores with spaces, capitalize first letter
	if field == "" {
		return field
	}
	result := make([]byte, 0, len(field))
	capitalize := true
	for i := 0; i < len(field); i++ {
		if field[i] == '_' {
			result = append(result, ' ')
			capitalize = true
		} else if capitalize {
			result = append(result, field[i]-32) // uppercase
			capitalize = false
		} else {
			result = append(result, field[i])
		}
	}
	return string(result)
}
