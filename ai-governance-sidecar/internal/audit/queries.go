package audit

const (
	queryInsertEntry = `
		INSERT INTO audit_log (tool_input, decision, reason) 
		VALUES (?, ?, ?)`

	querySelectAll = `
		SELECT id, timestamp, tool_input, decision, reason 
		FROM audit_log 
		ORDER BY timestamp DESC`

	timestampLayout = "2006-01-02 15:04:05"
)