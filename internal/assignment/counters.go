package assignment

import (
	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"gorm.io/gorm"
)

// agentLoad is a helper struct for scanning load counts.
type agentLoad struct {
	AgentID uuid.UUID `gorm:"column:agent_id"`
	Count   int64     `gorm:"column:count"`
}

// ChatLoadCounter counts active AgentTransfer records per agent.
func ChatLoadCounter(db *gorm.DB, orgID uuid.UUID, agentIDs []uuid.UUID) map[uuid.UUID]int64 {
	var loads []agentLoad
	db.Model(&models.AgentTransfer{}).
		Select("agent_id, COUNT(*) as count").
		Where("organization_id = ? AND agent_id IN ? AND status = ?", orgID, agentIDs, models.TransferStatusActive).
		Group("agent_id").
		Scan(&loads)

	loadMap := make(map[uuid.UUID]int64, len(loads))
	for _, l := range loads {
		loadMap[l.AgentID] = l.Count
	}
	return loadMap
}

// CallLoadCounter counts active CallTransfer records (waiting + connected) per agent.
func CallLoadCounter(db *gorm.DB, orgID uuid.UUID, agentIDs []uuid.UUID) map[uuid.UUID]int64 {
	var loads []agentLoad
	db.Model(&models.CallTransfer{}).
		Select("agent_id, COUNT(*) as count").
		Where("organization_id = ? AND agent_id IN ? AND status IN ?", orgID, agentIDs,
			[]models.CallTransferStatus{models.CallTransferStatusWaiting, models.CallTransferStatusConnected}).
		Group("agent_id").
		Scan(&loads)

	loadMap := make(map[uuid.UUID]int64, len(loads))
	for _, l := range loads {
		loadMap[l.AgentID] = l.Count
	}
	return loadMap
}

// IsAgentOnActiveCall checks whether an agent is currently on an active telephone call
// (either an answered incoming transfer or an active outgoing call).
func IsAgentOnActiveCall(db *gorm.DB, agentID uuid.UUID) bool {
	var transferCount int64
	db.Model(&models.CallTransfer{}).
		Where("agent_id = ? AND status = ?", agentID, models.CallTransferStatusConnected).
		Count(&transferCount)
	if transferCount > 0 {
		return true
	}

	var callLogCount int64
	db.Model(&models.CallLog{}).
		Where("agent_id = ? AND status IN (?, ?, ?)",
			agentID, models.CallStatusInitiating, models.CallStatusRinging, models.CallStatusAnswered).
		Count(&callLogCount)
	return callLogCount > 0
}
