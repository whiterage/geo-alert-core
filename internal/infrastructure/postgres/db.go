package postgres

import (
	"database/sql"
	"fmt"

	// Registers the "postgres" driver used by sql.Open below.
	_ "github.com/lib/pq"
)

func NewDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// pull sett
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	return db, nil
}
