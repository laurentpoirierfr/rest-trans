package transaction

import (
	"database/sql"
	"fmt"
	"log/slog"
)

const migrationSQL = `
CREATE TABLE IF NOT EXISTS rest_transactions (
    id VARCHAR(36) PRIMARY KEY,
    schema_name VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS rest_transaction_operations (
    id SERIAL PRIMARY KEY,
    transaction_id VARCHAR(36) NOT NULL REFERENCES rest_transactions(id) ON DELETE CASCADE,
    operation VARCHAR(10) NOT NULL,
    table_name TEXT NOT NULL,
    sql_query TEXT NOT NULL,
    params JSONB,
    before_state JSONB,
    committed_state JSONB,
    row_ids JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_rest_tx_ops_txid ON rest_transaction_operations(transaction_id);
CREATE INDEX IF NOT EXISTS idx_rest_tx_status ON rest_transactions(status);
`

const triggerFunctionSQL = `
CREATE OR REPLACE FUNCTION rest_notify() RETURNS trigger AS $$
DECLARE
    payload jsonb;
    channel text;
BEGIN
    channel := TG_TABLE_SCHEMA || '_' || TG_TABLE_NAME;

    IF TG_OP = 'INSERT' THEN
        payload := jsonb_build_object(
            'schema', TG_TABLE_SCHEMA,
            'table', TG_TABLE_NAME,
            'op', TG_OP,
            'new', to_jsonb(NEW)
        );
    ELSIF TG_OP = 'UPDATE' THEN
        payload := jsonb_build_object(
            'schema', TG_TABLE_SCHEMA,
            'table', TG_TABLE_NAME,
            'op', TG_OP,
            'old', to_jsonb(OLD),
            'new', to_jsonb(NEW)
        );
    ELSIF TG_OP = 'DELETE' THEN
        payload := jsonb_build_object(
            'schema', TG_TABLE_SCHEMA,
            'table', TG_TABLE_NAME,
            'op', TG_OP,
            'old', to_jsonb(OLD)
        );
    END IF;

    PERFORM pg_notify('rest_' || channel, payload::text);
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;
`

func Migrate(db *sql.DB) error {
	slog.Info("running transaction schema migration")

	if _, err := db.Exec(migrationSQL); err != nil {
		return fmt.Errorf("create transaction tables: %w", err)
	}

	slog.Info("transaction schema migration completed")
	return nil
}

func EnsureNotifyTrigger(db *sql.DB, schemaName, tableName string) error {
	triggerName := fmt.Sprintf("%s_%s_notify", tableName, tableName)
	query := fmt.Sprintf(`
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_trigger WHERE tgname = '%s'
			) THEN
				CREATE TRIGGER %s
					AFTER INSERT OR UPDATE OR DELETE ON %s.%s
					FOR EACH ROW EXECUTE FUNCTION rest_notify();
			END IF;
		END $$;
	`, triggerName, triggerName, schemaName, tableName)

	if _, err := db.Exec(triggerFunctionSQL); err != nil {
		return fmt.Errorf("create notify function: %w", err)
	}

	if _, err := db.Exec(query); err != nil {
		return fmt.Errorf("create trigger on %s.%s: %w", schemaName, tableName, err)
	}

	slog.Info("notify trigger ensured", "schema", schemaName, "table", tableName)
	return nil
}
