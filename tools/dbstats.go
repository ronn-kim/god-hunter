package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func main() {
	homeDir, _ := os.UserHomeDir()
	dbPath := filepath.Join(homeDir, ".god-hunter", "sessions.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Count records
	var sessionCount, chainCount, requestCount, findingCount int

	db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&sessionCount)
	db.QueryRow("SELECT COUNT(*) FROM chains").Scan(&chainCount)
	db.QueryRow("SELECT COUNT(*) FROM requests").Scan(&requestCount)
	db.QueryRow("SELECT COUNT(*) FROM findings").Scan(&findingCount)

	fmt.Printf("Database Statistics:\n")
	fmt.Printf("  Sessions: %d\n", sessionCount)
	fmt.Printf("  Chains: %d\n", chainCount)
	fmt.Printf("  Requests: %d\n", requestCount)
	fmt.Printf("  Findings: %d\n", findingCount)

	// Show recent sessions
	fmt.Printf("\nRecent Sessions:\n")
	rows, _ := db.Query("SELECT id, target_domain, program_name, status, created_at FROM sessions ORDER BY created_at DESC LIMIT 5")
	defer rows.Close()

	for rows.Next() {
		var id, targetDomain, programName, status, createdAt string
		if err := rows.Scan(&id, &targetDomain, &programName, &status, &createdAt); err != nil {
			continue
		}
		fmt.Printf("  [%s] %s (Domain: %s, Status: %s)\n", createdAt[:19], id, targetDomain, status)
	}
}
