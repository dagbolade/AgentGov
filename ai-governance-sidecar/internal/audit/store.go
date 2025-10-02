package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := openDatabase(dbPath)
	if err != nil {
		return nil, err
	}

	store := &SQLiteStore{db: db}
	
	if err := store.initializeSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

func (s *SQLiteStore) Log(ctx context.Context, toolInput json.RawMessage, decision Decision, reason string) error {
	if err := validateLogInput(toolInput, decision, reason); err != nil {
		return err
	}

	return s.insertEntry(ctx, toolInput, decision, reason)
}

func (s *SQLiteStore) GetAll(ctx context.Context) ([]Entry, error) {
	rows, err := s.queryAllEntries(ctx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanEntries(rows)
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) initializeSchema() error {
	for _, stmt := range schemaStatements() {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("execute schema: %w", err)
		}
	}
	return nil
}

func (s *SQLiteStore) insertEntry(ctx context.Context, toolInput json.RawMessage, decision Decision, reason string) error {
	const maxRetries = 3
	var err error
	
	for attempt := 0; attempt < maxRetries; attempt++ {
		_, err = s.db.ExecContext(ctx, queryInsertEntry, string(toolInput), string(decision), reason)
		if err == nil {
			return nil
		}
		
		// Check if it's a lock error
		if strings.Contains(err.Error(), "database is locked") || strings.Contains(err.Error(), "SQLITE_BUSY") {
			// Exponential backoff
			backoff := time.Duration(attempt+1) * 10 * time.Millisecond
			time.Sleep(backoff)
			continue
		}
		
		// Non-lock error, fail immediately
		return fmt.Errorf("insert entry: %w", err)
	}
	
	return fmt.Errorf("insert entry after %d retries: %w", maxRetries, err)
}

func (s *SQLiteStore) queryAllEntries(ctx context.Context) (*sql.Rows, error) {
	rows, err := s.db.QueryContext(ctx, querySelectAll)
	if err != nil {
		return nil, fmt.Errorf("query entries: %w", err)
	}
	return rows, nil
}