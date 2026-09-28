package db

import (
	"database/sql"
	"fmt"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

func Open(url, authToken string) (*sql.DB, error) {
	db, err := sql.Open("libsql", url+"?authToken="+authToken)

	if err != nil {
		return nil, fmt.Errorf("open turso %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping turso %w", err)
	}
	return db, nil
}

// func InsertTestTicket(db *sql.DB) error {
// 	_, err := db.Exec(
// 		"INSERT INTO Queue (id, name, created_at) VALUES (?, ?, ?)",
// 		"test-queue-1", "test-queue-name-1", time.Now().Format(time.RFC3339),
// 	)
// 	if err != nil {
// 		return fmt.Errorf("insert test queue: %w", err)
// 	}

// 	_, err = db.Exec(
// 		`INSERT INTO Ticket (id, queue_id, display_name, party_size, status, joined_at)
// 		 VALUES (?, ?, ?, ?, ?, ?)`,
// 		"test-ticket-1", "test-queue-1", "Sanity Check", 2, "waiting", time.Now().Format(time.RFC3339),
// 	)
// 	if err != nil {
// 		return fmt.Errorf("insert test ticket: %w", err)
// 	}
// 	return nil
// }

// func PrintAllTickets(db *sql.DB) error {
// 	rows, err := db.Query(`SELECT id, display_name, status FROM Ticket`)
// 	if err != nil {
// 		return fmt.Errorf("select tickets: %w", err)
// 	}
// 	defer rows.Close()

// 	for rows.Next() {
// 		var id, name, status string
// 		if err := rows.Scan(&id, &name, &status); err != nil {
// 			return fmt.Errorf("scan ticket %w", err)
// 		}
// 		fmt.Printf("ticket: id=%s, name=%s, status=%s\n", id, name, status)
// 	}
// 	return rows.Err()
// }
