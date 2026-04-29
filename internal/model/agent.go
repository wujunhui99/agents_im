package model

import "time"

const (
	AgentStatusDraft    = "draft"
	AgentStatusActive   = "active"
	AgentStatusDisabled = "disabled"
	AgentStatusArchived = "archived"
)

type Agent struct {
	AgentID     string
	IMUserID    string
	Name        string
	Description string
	Status      string
	CreatedBy   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (a Agent) Clone() Agent {
	return a
}
