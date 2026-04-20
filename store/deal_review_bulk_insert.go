package store

import "gorm.io/gorm"

const (
	// Keep wide deal-review bulk inserts comfortably below PostgreSQL's
	// 65535 bind-parameter limit and SQLite's host-parameter ceiling.
	dealReviewBulkInsertBatchSize = 200
)

func dealReviewCreateInBatches[T any](tx *gorm.DB, rows []T) error {
	if tx == nil || len(rows) == 0 {
		return nil
	}
	return tx.CreateInBatches(rows, dealReviewBulkInsertBatchSize).Error
}
