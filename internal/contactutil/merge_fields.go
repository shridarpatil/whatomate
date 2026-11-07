package contactutil

import (
	"sort"

	"github.com/nikyjain/whatomate/internal/models"
)

// mergedFields returns the survivor's column updates after folding in its
// duplicates. The survivor wins where it has a value; otherwise the most
// recently active duplicate fills the gap. Timestamps take the latest value,
// tags are unioned, opt-outs are sticky, and an agent-set name always wins.
func mergedFields(survivor models.Contact, dups []models.Contact) map[string]any {
	others := append([]models.Contact(nil), dups...)
	sort.SliceStable(others, func(i, j int) bool {
		return timeOrZero(others[i].LastMessageAt).After(timeOrZero(others[j].LastMessageAt))
	})
	all := append([]models.Contact{survivor}, others...)

	u := map[string]any{}

	name, locked := survivor.ProfileName, survivor.ProfileNameLocked
	if !locked {
		for _, c := range others {
			if c.ProfileNameLocked && c.ProfileName != "" {
				name, locked = c.ProfileName, true
				break
			}
		}
	}
	if name == "" {
		for _, c := range others {
			if c.ProfileName != "" {
				name = c.ProfileName
				break
			}
		}
	}
	u["profile_name"] = name
	u["profile_name_locked"] = locked

	if survivor.WhatsAppAccount == "" {
		for _, c := range others {
			if c.WhatsAppAccount != "" {
				u["whats_app_account"] = c.WhatsAppAccount
				break
			}
		}
	}
	if survivor.AssignedUserID == nil {
		for _, c := range others {
			if c.AssignedUserID != nil {
				u["assigned_user_id"] = *c.AssignedUserID
				break
			}
		}
	}
	if survivor.BSUID == "" {
		for _, c := range others {
			if c.BSUID != "" {
				u["bsuid"] = c.BSUID
				break
			}
		}
	}

	latest := survivor
	for _, c := range others {
		if timeOrZero(c.LastMessageAt).After(timeOrZero(latest.LastMessageAt)) {
			latest = c
		}
	}
	if latest.LastMessageAt != nil {
		u["last_message_at"] = *latest.LastMessageAt
		u["last_message_preview"] = latest.LastMessagePreview
	}

	isRead, optOut := true, false
	var lastInbound, chatbotLast = survivor.LastInboundAt, survivor.ChatbotLastMessageAt
	for _, c := range all {
		isRead = isRead && c.IsRead
		optOut = optOut || c.MarketingOptOut
		if timeOrZero(c.LastInboundAt).After(timeOrZero(lastInbound)) {
			lastInbound = c.LastInboundAt
		}
		if timeOrZero(c.ChatbotLastMessageAt).After(timeOrZero(chatbotLast)) {
			chatbotLast = c.ChatbotLastMessageAt
		}
	}
	u["is_read"] = isRead
	u["marketing_opt_out"] = optOut
	if lastInbound != nil {
		u["last_inbound_at"] = *lastInbound
	}
	if chatbotLast != nil {
		u["chatbot_last_message_at"] = *chatbotLast
	}

	u["tags"] = unionTags(all)
	u["metadata"] = mergeMetadata(survivor, others)
	return u
}

func unionTags(contacts []models.Contact) models.JSONBArray {
	seen := map[string]bool{}
	out := models.JSONBArray{}
	for _, c := range contacts {
		for _, t := range c.Tags {
			s, ok := t.(string)
			if !ok || s == "" || seen[s] {
				continue
			}
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// mergeMetadata layers duplicates oldest-activity first, then the survivor on
// top, so the survivor's keys win.
func mergeMetadata(survivor models.Contact, others []models.Contact) models.JSONB {
	out := models.JSONB{}
	for i := len(others) - 1; i >= 0; i-- {
		for k, v := range others[i].Metadata {
			out[k] = v
		}
	}
	for k, v := range survivor.Metadata {
		out[k] = v
	}
	return out
}
