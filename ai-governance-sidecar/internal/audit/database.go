package audit

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
)

func openDatabase(dbPath string) (*sql.DB, error) {
	if err := ensureDBDirectory(dbPath); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Set connection pool limits for concurrent access
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(0)


	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if err := configureSQLite(db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func configureSQLite(db *sql.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",           // Write-Ahead Logging for better concurrency
		"PRAGMA synchronous=NORMAL",         // Balance between safety and performance
		"PRAGMA busy_timeout=5000",          // Wait 5s on lock before failing
		"PRAGMA cache_size=-64000",          // 64MB cache
		"PRAGMA foreign_keys=ON",            // Enable foreign key constraints
		"PRAGMA temp_store=MEMORY",          // Store temp tables in memory
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("execute pragma: %w", err)
		}
	}

	return nil
}

func ensureDBDirectory(dbPath string) error {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create db directory: %w", err)
	}
	return nil
}