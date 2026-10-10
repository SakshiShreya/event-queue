package db

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"strings"
	"time"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

//go:embed schema.sql
var schema string

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

func Migrate(ctx context.Context, db *sql.DB) error {
	statements := strings.Split(schema, ";")

	for _, statement := range statements {
		statement := strings.TrimSpace(statement)
		if statement == "" {
			continue
		}

		_, err := db.ExecContext(ctx, statement)

		if err != nil {
			return fmt.Errorf("exec %q: %w", statement, err)
		}
	}

	now := time.Now().Unix()
	_, err := db.ExecContext(
		ctx,
		"INSERT OR IGNORE INTO Rooms (id, name, created_at, updated_at) VALUES ('demo', 'Demo', ?, ?)",
		now, now,
	)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	return nil
}
