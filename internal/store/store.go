package store

import (
	"database/sql"
	"fmt"

	_ "github.com/marcboeker/go-duckdb"
)

// Store wraps a DuckDB connection.
type Store struct {
	db *sql.DB
}

// New opens (or creates) a DuckDB database and applies the schema.
func New(path string) (*Store, error) {
	db, err := sql.Open("duckdb", path)
	if err != nil {
		return nil, fmt.Errorf("open duckdb %s: %w", path, err)
	}

	if _, err := db.Exec(DDL); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	return &Store{db: db}, nil
}

// NewReadOnly opens a DuckDB database in read-only mode (no locking conflicts).
func NewReadOnly(path string) (*Store, error) {
	dsn := path + "?access_mode=read_only"
	db, err := sql.Open("duckdb", dsn)
	if err != nil {
		return nil, fmt.Errorf("open duckdb read-only %s: %w", path, err)
	}

	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying sql.DB for direct queries.
func (s *Store) DB() *sql.DB {
	return s.db
}
