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
	AccountID   string
	IMUserID    string
	Name        string
	Description string
	Status      string
	CreatedBy   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (a Agent) Clone() Agent {
	if a.AccountID == "" {
		a.AccountID = a.IMUserID
	}
	if a.IMUserID == "" {
		a.IMUserID = a.AccountID
	}
	return a
}
