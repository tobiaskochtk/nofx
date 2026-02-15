// fix_trader_userid.go - Update trader UserID from UUID to "Test"
package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

func main() {
	// Open database
	db, err := sql.Open("sqlite", "/data/nofx.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Check current trader data
	fmt.Println("Current trader data:")
	rows, err := db.Query("SELECT id, user_id, name FROM traders WHERE id LIKE 'hyperliquid%'")
	if err != nil {
		log.Fatalf("Failed to query traders: %v", err)
	}
	for rows.Next() {
		var id, userID, name string
		rows.Scan(&id, &userID, &name)
		fmt.Printf("  ID: %s, UserID: %s, Name: %s\n", id, userID, name)
	}
	rows.Close()

	// Update trader user_id
	result, err := db.Exec("UPDATE traders SET user_id = 'Test' WHERE id LIKE 'hyperliquid_0b168b37-04c4-444e-95d9-2c50e231e823%'")
	if err != nil {
		log.Fatalf("Failed to update trader: %v", err)
	}

	affected, _ := result.RowsAffected()
	fmt.Printf("\n✓ Updated %d trader(s)\n\n", affected)

	// Verify the update
	fmt.Println("After update:")
	rows2, err := db.Query("SELECT id, user_id, name FROM traders WHERE id LIKE 'hyperliquid%'")
	if err != nil {
		log.Fatalf("Failed to query traders: %v", err)
	}
	for rows2.Next() {
		var id, userID, name string
		rows2.Scan(&id, &userID, &name)
		fmt.Printf("  ID: %s, UserID: %s, Name: %s\n", id, userID, name)
	}
	rows2.Close()
}
