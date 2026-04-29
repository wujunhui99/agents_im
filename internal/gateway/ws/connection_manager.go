package ws

import (
	"sync"
	"time"
)

type ConnectionInfo struct {
	ConnectionID string
	UserID       string
	ConnectedAt  time.Time
	LastSeenAt   time.Time
}

type ConnectionManager struct {
	mu          sync.RWMutex
	byID        map[string]*Connection
	byUserID    map[string]map[string]*Connection
	presenceLog PresenceReporter
}

type PresenceReporter interface {
	Connected(info ConnectionInfo)
	Disconnected(info ConnectionInfo)
}

func NewConnectionManager() *ConnectionManager {
	return NewConnectionManagerWithPresence(nil)
}

func NewConnectionManagerWithPresence(presenceLog PresenceReporter) *ConnectionManager {
	return &ConnectionManager{
		byID:        make(map[string]*Connection),
		byUserID:    make(map[string]map[string]*Connection),
		presenceLog: presenceLog,
	}
}

func (m *ConnectionManager) Register(conn *Connection) {
	if conn == nil {
		return
	}

	m.mu.Lock()
	m.byID[conn.ID] = conn
	userConnections := m.byUserID[conn.UserID]
	if userConnections == nil {
		userConnections = make(map[string]*Connection)
		m.byUserID[conn.UserID] = userConnections
	}
	userConnections[conn.ID] = conn
	m.mu.Unlock()

	if m.presenceLog != nil {
		m.presenceLog.Connected(conn.Info())
	}
}

func (m *ConnectionManager) Unregister(connectionID string) {
	m.mu.Lock()
	conn := m.byID[connectionID]
	if conn != nil {
		delete(m.byID, connectionID)
		if userConnections := m.byUserID[conn.UserID]; userConnections != nil {
			delete(userConnections, connectionID)
			if len(userConnections) == 0 {
				delete(m.byUserID, conn.UserID)
			}
		}
	}
	m.mu.Unlock()

	if conn != nil && m.presenceLog != nil {
		m.presenceLog.Disconnected(conn.Info())
	}
}

func (m *ConnectionManager) Connection(connectionID string) (*Connection, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conn, ok := m.byID[connectionID]
	return conn, ok
}

func (m *ConnectionManager) UserConnectionIDs(userID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	userConnections := m.byUserID[userID]
	ids := make([]string, 0, len(userConnections))
	for connectionID := range userConnections {
		ids = append(ids, connectionID)
	}
	return ids
}

func (m *ConnectionManager) UserConnections(userID string) []*Connection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	userConnections := m.byUserID[userID]
	connections := make([]*Connection, 0, len(userConnections))
	for _, conn := range userConnections {
		connections = append(connections, conn)
	}
	return connections
}

func (m *ConnectionManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.byID)
}

func (m *ConnectionManager) UserCount(userID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.byUserID[userID])
}
