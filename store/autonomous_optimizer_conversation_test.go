package store

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	_ "modernc.org/sqlite"
)

func TestAutonomousOptimizerConversationRoundTrip(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "autonomous-optimizer-conversation.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root, err := NewFromGorm(gdb)
	if err != nil {
		t.Fatalf("NewFromGorm() error = %v", err)
	}
	if err := gdb.AutoMigrate(&AutonomousOptimizerConversation{}, &AutonomousOptimizerConversationMessage{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	conversation, err := root.AutonomousOptimizer().GetOrCreateConversation(
		"user-1",
		"trader-1",
		AutonomousOptimizerConversationPurposeProposal,
		"model-1",
		"gpt-5.4",
	)
	if err != nil {
		t.Fatalf("GetOrCreateConversation() error = %v", err)
	}
	if conversation == nil {
		t.Fatal("GetOrCreateConversation() = nil")
	}

	if err := root.AutonomousOptimizer().AppendConversationMessages(conversation, "run-1", []*AutonomousOptimizerConversationMessage{
		{
			Role:          AutonomousOptimizerConversationRoleSystem,
			Content:       "system-full",
			ReplayContent: "",
		},
		{
			Role:          AutonomousOptimizerConversationRoleUser,
			Content:       "user-full",
			ReplayContent: "user-replay",
		},
		{
			Role:          AutonomousOptimizerConversationRoleAssistant,
			Content:       "assistant-full",
			ReplayContent: "assistant-replay",
		},
	}); err != nil {
		t.Fatalf("AppendConversationMessages() error = %v", err)
	}

	messages, err := root.AutonomousOptimizer().ListConversationMessages(conversation.ID, 6)
	if err != nil {
		t.Fatalf("ListConversationMessages() error = %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("len(messages) = %d, want 3", len(messages))
	}
	if messages[0].Sequence != 1 || messages[1].Sequence != 2 || messages[2].Sequence != 3 {
		t.Fatalf("sequences = %#v, want ascending 1..3", []int{messages[0].Sequence, messages[1].Sequence, messages[2].Sequence})
	}
	if messages[1].ReplayContent != "user-replay" || messages[2].ReplayContent != "assistant-replay" {
		t.Fatalf("replay content = %#v / %#v, want persisted replay fields", messages[1].ReplayContent, messages[2].ReplayContent)
	}
	if conversation.LastRunID != "run-1" {
		t.Fatalf("LastRunID = %q, want run-1", conversation.LastRunID)
	}
	if conversation.LastMessageAt.IsZero() {
		t.Fatal("LastMessageAt is zero, want append timestamp")
	}
}
