package audit

const (
	tableSchema = `
		CREATE TABLE IF NOT EXISTS audit_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			tool_input TEXT NOT NULL,
			decision TEXT NOT NULL CHECK(decision IN ('allow', 'deny')),
			reason TEXT NOT NULL
		)`

	triggerPreventUpdate = `
		CREATE TRIGGER IF NOT EXISTS prevent_update
		BEFORE UPDATE ON audit_log
		FOR EACH ROW
		BEGIN
			SELECT RAISE(FAIL, 'Updates not allowed on audit_log');
		END`

	triggerPreventDelete = `
		CREATE TRIGGER IF NOT EXISTS prevent_delete
		BEFORE DELETE ON audit_log
		FOR EACH ROW
		BEGIN
			SELECT RAISE(FAIL, 'Deletes not allowed on audit_log');
		END`

	indexTimestamp = `
		CREATE INDEX IF NOT EXISTS idx_timestamp ON audit_log(timestamp DESC)`
)

func schemaStatements() []string {
	return []string{
		tableSchema,
		triggerPreventUpdate,
		triggerPreventDelete,
		indexTimestamp,
	}
}