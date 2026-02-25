package calling

import (
	"context"
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
)

// runIVRFlow executes the IVR menu tree for an active call session
func (m *Manager) runIVRFlow(session *CallSession, waAccount *whatsapp.Account) {
	if session.IVRFlow == nil || session.IVRFlow.Menu == nil {
		m.log.Info("No IVR flow or menu configured", "call_id", session.ID)
		return
	}

	// Parse the menu JSON into our tree structure
	var rootMenu IVRMenuNode
	menuBytes, err := json.Marshal(session.IVRFlow.Menu)
	if err != nil {
		m.log.Error("Failed to marshal IVR menu", "error", err, "call_id", session.ID)
		return
	}
	if err := json.Unmarshal(menuBytes, &rootMenu); err != nil {
		m.log.Error("Failed to parse IVR menu", "error", err, "call_id", session.ID)
		return
	}

	// Set defaults
	if rootMenu.TimeoutSeconds == 0 {
		rootMenu.TimeoutSeconds = 10
	}
	if rootMenu.MaxRetries == 0 {
		rootMenu.MaxRetries = 3
	}

	// Set parent references for submenus
	setMenuParents(&rootMenu, nil)

	// Track path through IVR — load existing steps so goto_flow accumulates
	var ivrPath []map[string]string
	var existingLog models.CallLog
	if err := m.db.Select("ivr_path").Where("id = ?", session.CallLogID).First(&existingLog).Error; err == nil {
		if existingLog.IVRPath != nil {
			if steps, ok := existingLog.IVRPath["steps"].([]interface{}); ok {
				for _, s := range steps {
					if stepMap, ok := s.(map[string]interface{}); ok {
						entry := map[string]string{}
						for k, v := range stepMap {
							if str, ok := v.(string); ok {
								entry[k] = str
							}
						}
						ivrPath = append(ivrPath, entry)
					}
				}
			}
		}
	}

	// Record which flow we're entering (first call gets flow_start,
	// subsequent goto_flow calls already have a goto_flow marker)
	if len(ivrPath) == 0 {
		ivrPath = append(ivrPath, map[string]string{"action": "flow_start", "flow": session.IVRFlow.Name})
	}

	// Start at the root menu
	currentMenu := &rootMenu
	session.mu.Lock()
	session.CurrentMenu = currentMenu
	session.mu.Unlock()

	// Reuse the session's IVR player to maintain RTP sequence continuity
	// across goto_flow transitions (new player would reset seq to 0,
	// causing the receiver to drop packets as duplicates).
	session.mu.Lock()
	if session.IVRPlayer == nil {
		session.IVRPlayer = NewAudioPlayer(session.AudioTrack)
	}
	player := session.IVRPlayer
	session.mu.Unlock()

	for {
		// Check if session is still active
		session.mu.Lock()
		status := session.Status
		session.mu.Unlock()

		if status != models.CallStatusAnswered {
			break
		}

		// Play greeting audio if available
		if currentMenu.Greeting != "" && m.config.AudioDir != "" {
			audioFile := filepath.Join(m.config.AudioDir, currentMenu.Greeting)
			m.log.Info("Playing IVR greeting", "call_id", session.ID, "file", audioFile)
			packets, err := player.PlayFile(audioFile)
			if err != nil {
				m.log.Error("Failed to play greeting audio", "error", err, "call_id", session.ID, "file", audioFile)
			} else {
				m.log.Info("IVR greeting playback finished", "call_id", session.ID, "packets_sent", packets)
			}
		} else {
			m.log.Warn("No greeting configured for IVR menu", "call_id", session.ID, "greeting", currentMenu.Greeting, "audio_dir", m.config.AudioDir)
		}

		// Wait for DTMF input
		digit, ok := m.waitForDTMF(session, time.Duration(currentMenu.TimeoutSeconds)*time.Second, currentMenu.MaxRetries)
		if !ok {
			// Timeout or max retries exceeded - hang up
			m.log.Info("IVR timeout/max retries, terminating call", "call_id", session.ID)
			m.terminateCall(session, waAccount)
			break
		}

		digitStr := string(digit)

		// Look up the option for this digit
		option, exists := currentMenu.Options[digitStr]
		if !exists {
			// Invalid input - play error and retry
			m.log.Info("Invalid IVR input", "call_id", session.ID, "digit", digitStr)
			continue
		}

		m.log.Info("IVR option selected",
			"call_id", session.ID,
			"digit", digitStr,
			"action", option.Action,
			"label", option.Label,
		)

		// Record the digit step (goto_flow merges digit into its flow marker below)
		if option.Action != "goto_flow" {
			ivrPath = append(ivrPath, map[string]string{"digit": digitStr, "label": option.Label, "action": option.Action})
		}

		switch option.Action {
		case "submenu":
			if option.Menu != nil {
				setMenuParents(option.Menu, currentMenu)
				currentMenu = option.Menu
				session.mu.Lock()
				session.CurrentMenu = currentMenu
				session.mu.Unlock()
			}

		case "parent":
			if currentMenu.Parent != nil {
				currentMenu = currentMenu.Parent
				session.mu.Lock()
				session.CurrentMenu = currentMenu
				session.mu.Unlock()
			}

		case "repeat":
			// Loop back to play the same menu again
			continue

		case "transfer":
			m.saveIVRPath(session, ivrPath)
			m.initiateTransfer(session, session.AccountName, option.Target, ivrPath)
			return

		case "goto_flow":
			if option.Target != "" {
				m.log.Info("IVR goto_flow", "call_id", session.ID, "target_flow", option.Target)

				targetFlowID, err := uuid.Parse(option.Target)
				if err != nil {
					m.log.Error("Invalid goto_flow target ID", "error", err, "call_id", session.ID)
					continue
				}

				var targetFlow models.IVRFlow
				if err := m.db.First(&targetFlow, targetFlowID).Error; err != nil {
					m.log.Error("Failed to load goto_flow target", "error", err, "call_id", session.ID, "flow_id", option.Target)
					continue
				}

				if !targetFlow.IsActive {
					m.log.Warn("goto_flow target is disabled, skipping", "call_id", session.ID, "flow_id", option.Target)
					continue
				}

				// Record the flow transition in the IVR path (include digit so the tree shows which key was pressed)
				ivrPath = append(ivrPath, map[string]string{"action": "goto_flow", "flow": targetFlow.Name, "digit": digitStr, "label": option.Label})
				m.saveIVRPath(session, ivrPath)

				// Switch to the new flow and restart the IVR loop
				session.mu.Lock()
				session.IVRFlow = &targetFlow
				session.mu.Unlock()

				// Update the call log to reference the new flow
				m.db.Model(&models.CallLog{}).
					Where("id = ?", session.CallLogID).
					Update("ivr_flow_id", targetFlow.ID)

				m.runIVRFlow(session, waAccount)
				return
			}

		case "hangup":
			m.saveIVRPath(session, ivrPath)
			m.terminateCall(session, waAccount)
			return
		}
	}

	// Save the IVR path to the call log
	m.saveIVRPath(session, ivrPath)
}

// waitForDTMF waits for a DTMF digit with timeout and retries
func (m *Manager) waitForDTMF(session *CallSession, timeout time.Duration, maxRetries int) (byte, bool) {
	for attempt := 0; attempt < maxRetries; attempt++ {
		select {
		case digit, ok := <-session.DTMFBuffer:
			if !ok {
				return 0, false // Channel closed, session ending
			}
			return digit, true
		case <-time.After(timeout):
			m.log.Debug("DTMF timeout", "call_id", session.ID, "attempt", attempt+1)
		}
	}
	return 0, false
}

// saveIVRPath saves the recorded IVR navigation path to the call log
func (m *Manager) saveIVRPath(session *CallSession, path []map[string]string) {
	if len(path) == 0 {
		return
	}

	pathJSON := models.JSONB{}
	pathJSON["steps"] = path

	m.db.Model(&models.CallLog{}).
		Where("id = ?", session.CallLogID).
		Update("ivr_path", pathJSON)
}

// terminateCall terminates an active call via the WhatsApp API
func (m *Manager) terminateCall(session *CallSession, waAccount *whatsapp.Account) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := m.whatsapp.TerminateCall(ctx, waAccount, session.ID); err != nil {
		m.log.Error("Failed to terminate call via API", "error", err, "call_id", session.ID)
	}
}

// setMenuParents recursively sets parent references for IVR menu nodes
func setMenuParents(menu *IVRMenuNode, parent *IVRMenuNode) {
	menu.Parent = parent
	for _, opt := range menu.Options {
		if opt.Menu != nil {
			setMenuParents(opt.Menu, menu)
		}
	}
}
