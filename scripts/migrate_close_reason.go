// migrate_close_reason.go - Add close_reason column to deals table
// Run: go run scripts/migrate_close_reason.go
package main

import (
	"database/sql"
	"log"
	"os"

	_ "modernc.org/sqlite"
)

func main() {
	// Get database path from args or use default
	dbPath := "nofx.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}

	log.Printf("🔧 Opening database: %s", dbPath)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("❌ Failed to open database: %v", err)
	}
	defer db.Close()

	// Check if column already exists
	log.Println("🔍 Checking if close_reason column exists...")
	var columnExists bool
	row := db.QueryRow(`
		SELECT COUNT(*) 
		FROM pragma_table_info('deals') 
		WHERE name = 'close_reason'
	`)
	if err := row.Scan(&columnExists); err != nil {
		log.Fatalf("❌ Failed to check column: %v", err)
	}

	if columnExists {
		log.Println("✓ Column close_reason already exists, skipping migration")
		return
	}

	// Add column
	log.Println("➕ Adding close_reason column to deals table...")
	_, err = db.Exec(`ALTER TABLE deals ADD COLUMN close_reason TEXT`)
	if err != nil {
		log.Fatalf("❌ Failed to add column: %v", err)
	}

	log.Println("✓ Column added successfully")

	// Update existing closed deals
	log.Println("📝 Updating existing closed deals with default reason...")
	result, err := db.Exec(`
		UPDATE deals 
		SET close_reason = 'unknown' 
		WHERE status = 'closed' AND close_reason IS NULL
	`)
	if err != nil {
		log.Printf("⚠️ Failed to update existing deals: %v", err)
	} else {
		affected, _ := result.RowsAffected()
		log.Printf("✓ Updated %d existing deals with default reason", affected)
	}

	// Show statistics
	var totalDeals, dealsWithReason, dealsWithoutReason int
	row = db.QueryRow(`
		SELECT 
			COUNT(*) as total,
			COUNT(close_reason) as with_reason,
			COUNT(*) - COUNT(close_reason) as without_reason
		FROM deals
	`)
	if err := row.Scan(&totalDeals, &dealsWithReason, &dealsWithoutReason); err != nil {
		log.Printf("⚠️ Failed to get statistics: %v", err)
	} else {
		log.Println("\n📊 Migration Statistics:")
		log.Printf("   Total deals: %d", totalDeals)
		log.Printf("   With close_reason: %d", dealsWithReason)
		log.Printf("   Without close_reason: %d", dealsWithoutReason)
	}

	log.Println("\n✅ Migration completed successfully!")
}
