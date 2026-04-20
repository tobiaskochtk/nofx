package store

import (
	"nofx/codexaudit"

	"gorm.io/gorm"
)

const CodexCallLogRetention = codexaudit.Retention

type CodexCallLog = codexaudit.LogEntry

type CodexCallLogStore struct {
	audit *codexaudit.Store
}

func NewCodexCallLogStore(db *gorm.DB) *CodexCallLogStore {
	codexaudit.SetDB(db)
	return &CodexCallLogStore{audit: codexaudit.NewStore(db)}
}

func (s *CodexCallLogStore) initTables() error {
	return s.audit.InitTables()
}

func (s *CodexCallLogStore) Record(entry *CodexCallLog) error {
	return s.audit.Record(entry)
}

func (s *CodexCallLogStore) ListRecentForUser(userID, traderID string, limit int) ([]CodexCallLog, error) {
	return s.audit.ListRecentForUser(userID, traderID, limit)
}
