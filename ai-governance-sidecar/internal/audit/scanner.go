package audit

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

func scanEntries(rows *sql.Rows) ([]Entry, error) {
	var entries []Entry

	for rows.Next() {
		entry, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return entries, nil
}

func scanEntry(rows *sql.Rows) (Entry, error) {
	var e Entry
	var timestamp string
	var toolInput string

	if err := rows.Scan(&e.ID, &timestamp, &toolInput, &e.Decision, &e.Reason); err != nil {
		return Entry{}, fmt.Errorf("scan row: %w", err)
	}

	parsedTime, err := parseTimestamp(timestamp)
	if err != nil {
		return Entry{}, err
	}
	e.Timestamp = parsedTime

	e.ToolInput = json.RawMessage(toolInput)

	return e, nil
}

func parseTimestamp(timestamp string) (time.Time, error) {
	// Try RFC3339 format first (SQLite default with CURRENT_TIMESTAMP)
	t, err := time.Parse(time.RFC3339, timestamp)
	if err == nil {
		return t, nil
	}

	// Fallback to SQLite datetime format
	t, err = time.Parse("2006-01-02 15:04:05", timestamp)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse timestamp: %w", err)
	}

	return t, nil
}