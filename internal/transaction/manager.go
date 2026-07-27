package transaction

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Manager struct {
	DB       *sql.DB
	TTL      time.Duration
	stopCh   chan struct{}
}

func NewManager(db *sql.DB, ttl time.Duration) *Manager {
	return &Manager{
		DB:     db,
		TTL:    ttl,
		stopCh: make(chan struct{}),
	}
}

func (m *Manager) Start(schemaName string) (*Transaction, error) {
	tx := &Transaction{
		ID:        uuid.New().String(),
		Schema:    schemaName,
		Status:    StatusPending,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	_, err := m.DB.Exec(`
		INSERT INTO rest_transactions (id, schema_name, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)`,
		tx.ID, tx.Schema, tx.Status, tx.CreatedAt, tx.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create transaction: %w", err)
	}

	return tx, nil
}

func (m *Manager) Get(txID string) (*Transaction, error) {
	tx := &Transaction{}
	err := m.DB.QueryRow(`
		SELECT id, schema_name, status, created_at, updated_at
		FROM rest_transactions WHERE id = $1`, txID,
	).Scan(&tx.ID, &tx.Schema, &tx.Status, &tx.CreatedAt, &tx.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get transaction: %w", err)
	}

	countRow := m.DB.QueryRow(`
		SELECT COUNT(*) FROM rest_transaction_operations WHERE transaction_id = $1`, txID)
	countRow.Scan(&tx.OpCount)

	return tx, nil
}

func (m *Manager) GetOperations(txID string) ([]Operation, error) {
	rows, err := m.DB.Query(`
		SELECT id, transaction_id, operation, table_name, sql_query, 
		       COALESCE(params, '[]'::jsonb), before_state, row_ids, created_at
		FROM rest_transaction_operations WHERE transaction_id = $1 ORDER BY id`, txID)
	if err != nil {
		return nil, fmt.Errorf("get operations: %w", err)
	}
	defer rows.Close()

	var ops []Operation
	for rows.Next() {
		var op Operation
		if err := rows.Scan(&op.ID, &op.TransactionID, &op.Operation, &op.TableName,
			&op.SQLQuery, &op.Params, &op.BeforeState, &op.RowIDs, &op.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan operation: %w", err)
		}
		ops = append(ops, op)
	}
	return ops, rows.Err()
}

func (m *Manager) List(schemaName string) ([]Transaction, error) {
	rows, err := m.DB.Query(`
		SELECT t.id, t.schema_name, t.status, t.created_at, t.updated_at,
		       (SELECT COUNT(*) FROM rest_transaction_operations o WHERE o.transaction_id = t.id)
		FROM rest_transactions t
		WHERE t.schema_name = $1 AND t.status = $2
		ORDER BY t.created_at DESC`, schemaName, StatusPending,
	)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	defer rows.Close()

	var txs []Transaction
	for rows.Next() {
		var tx Transaction
		if err := rows.Scan(&tx.ID, &tx.Schema, &tx.Status, &tx.CreatedAt, &tx.UpdatedAt, &tx.OpCount); err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		txs = append(txs, tx)
	}
	return txs, rows.Err()
}

func (m *Manager) Stage(txID, op, tableName, sqlQuery string, params []interface{}, beforeState []byte, rowIDs []byte) error {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("marshal params: %w", err)
	}

	tx, err := m.Get(txID)
	if err != nil {
		return err
	}
	if tx == nil || tx.Status != StatusPending {
		return fmt.Errorf("transaction not found or not pending")
	}

	_, err = m.DB.Exec(`
		INSERT INTO rest_transaction_operations (transaction_id, operation, table_name, sql_query, params, before_state, row_ids)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		txID, op, tableName, sqlQuery, paramsJSON, beforeState, rowIDs,
	)
	if err != nil {
		return fmt.Errorf("stage operation: %w", err)
	}

	return nil
}

func (m *Manager) Commit(txID string) error {
	tx, err := m.Get(txID)
	if err != nil {
		return err
	}
	if tx == nil || tx.Status != StatusPending {
		return fmt.Errorf("transaction not found or not pending")
	}

	lockTx, err := m.DB.BeginTx(context.Background(), &sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})
	if err != nil {
		return fmt.Errorf("begin lock transaction: %w", err)
	}
	defer lockTx.Rollback()

	_, err = lockTx.Exec(`UPDATE rest_transactions SET status = $1 WHERE id = $2 AND status = $3`,
		"committing", txID, StatusPending)
	if err != nil {
		return fmt.Errorf("lock transaction: %w", err)
	}

	rows, err := lockTx.Query(`
		SELECT id, operation, table_name, sql_query, COALESCE(params, '[]'::jsonb),
		       before_state, row_ids
		FROM rest_transaction_operations WHERE transaction_id = $1 ORDER BY id`, txID)
	if err != nil {
		return fmt.Errorf("read operations: %w", err)
	}

	type stagedOp struct {
		id          int
		op          string
		tableName   string
		sqlQuery    string
		params      string
		beforeState []byte
		rowIDs      []byte
	}
	var ops []stagedOp
	for rows.Next() {
		var op stagedOp
		if err := rows.Scan(&op.id, &op.op, &op.tableName, &op.sqlQuery, &op.params,
			&op.beforeState, &op.rowIDs); err != nil {
			rows.Close()
			return fmt.Errorf("scan operation: %w", err)
		}
		ops = append(ops, op)
	}
	rows.Close()

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iter operations: %w", err)
	}

	targetDB, err := m.resolveTargetDB(tx.Schema)
	if err != nil {
		return err
	}

	targetTx, err := targetDB.BeginTx(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("begin target transaction: %w", err)
	}
	defer targetTx.Rollback()

	for _, op := range ops {
		var params []interface{}
		if err := json.Unmarshal([]byte(op.params), &params); err != nil {
			return fmt.Errorf("unmarshal params for op %d: %w", op.id, err)
		}

		if op.op == "INSERT" {
			returningQuery := op.sqlQuery + " RETURNING id"
			var insertedID int
			if len(params) > 0 {
				err = targetTx.QueryRow(returningQuery, params...).Scan(&insertedID)
			} else {
				err = targetTx.QueryRow(returningQuery).Scan(&insertedID)
			}
			if err != nil {
				return fmt.Errorf("execute op %d (%s): %w", op.id, op.op, err)
			}
			rowIDsJSON, _ := json.Marshal([]interface{}{insertedID})
			_, _ = m.DB.Exec(`UPDATE rest_transaction_operations SET row_ids = $1 WHERE id = $2`,
				rowIDsJSON, op.id)
		} else {
			if len(params) > 0 {
				_, err = targetTx.Exec(op.sqlQuery, params...)
			} else {
				_, err = targetTx.Exec(op.sqlQuery)
			}
			if err != nil {
				return fmt.Errorf("execute op %d (%s): %w", op.id, op.op, err)
			}
		}
	}

	if err := targetTx.Commit(); err != nil {
		return fmt.Errorf("commit target: %w", err)
	}

	for _, op := range ops {
		if op.beforeState == nil {
			continue
		}
		var snapshots []map[string]interface{}
		if err := json.Unmarshal(op.beforeState, &snapshots); err != nil {
			continue
		}
		var committedSnapshots []map[string]interface{}
		for _, snapshot := range snapshots {
			idVal, ok := snapshot["id"]
			if !ok {
				continue
			}
			checkQuery := fmt.Sprintf("SELECT row_to_json(t) FROM (SELECT * FROM %s WHERE id = $1) t", op.tableName)
			var currentJSON string
			if err := m.DB.QueryRow(checkQuery, idVal).Scan(&currentJSON); err != nil {
				continue
			}
			var current map[string]interface{}
			if err := json.Unmarshal([]byte(currentJSON), &current); err != nil {
				continue
			}
			committedSnapshots = append(committedSnapshots, current)
		}
		if len(committedSnapshots) > 0 {
			committedJSON, _ := json.Marshal(committedSnapshots)
			m.DB.Exec(`UPDATE rest_transaction_operations SET committed_state = $1 WHERE id = $2`,
				committedJSON, op.id)
		}
	}

	now := time.Now().UTC()
	_, err = lockTx.Exec(`UPDATE rest_transactions SET status = $1, updated_at = $2 WHERE id = $3`,
		StatusCommitted, now, txID)
	if err != nil {
		return fmt.Errorf("mark committed: %w", err)
	}

	if err := lockTx.Commit(); err != nil {
		return fmt.Errorf("commit lock: %w", err)
	}

	return nil
}

func (m *Manager) Rollback(txID string) error {
	tx, err := m.Get(txID)
	if err != nil {
		return err
	}
	if tx == nil || tx.Status != StatusPending {
		return fmt.Errorf("transaction not found or not pending")
	}

	_, err = m.DB.Exec(`DELETE FROM rest_transaction_operations WHERE transaction_id = $1`, txID)
	if err != nil {
		return fmt.Errorf("delete operations: %w", err)
	}

	now := time.Now().UTC()
	_, err = m.DB.Exec(`UPDATE rest_transactions SET status = $1, updated_at = $2 WHERE id = $3`,
		StatusRolledBack, now, txID)
	if err != nil {
		return fmt.Errorf("mark rolled back: %w", err)
	}

	return nil
}

func (m *Manager) CaptureSnapshot(schemaName, tableName, whereClause string, params []interface{}) ([]byte, []byte, error) {
	query := fmt.Sprintf("SELECT * FROM %s.%s WHERE %s", schemaName, tableName, whereClause)
	rows, err := m.DB.Query(query, params...)
	if err != nil {
		return nil, nil, fmt.Errorf("capture snapshot query: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, fmt.Errorf("get columns: %w", err)
	}

	var snapshots []map[string]interface{}
	var rowIDs []interface{}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, nil, fmt.Errorf("scan row: %w", err)
		}

		row := make(map[string]interface{})
		var idVal interface{}
		for i, col := range columns {
			val := values[i]
			switch v := val.(type) {
			case []byte:
				row[col] = string(v)
				if col == "id" {
					idVal = string(v)
				}
			default:
				row[col] = v
				if col == "id" {
					idVal = v
				}
			}
		}
		snapshots = append(snapshots, row)
		if idVal != nil {
			rowIDs = append(rowIDs, idVal)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate rows: %w", err)
	}

	if len(snapshots) == 0 {
		return nil, nil, nil
	}

	snapshotJSON, err := json.Marshal(snapshots)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal snapshot: %w", err)
	}

	rowIDsJSON, err := json.Marshal(rowIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal row IDs: %w", err)
	}

	return snapshotJSON, rowIDsJSON, nil
}

func (m *Manager) RollbackPostCommit(txID string) error {
	tx, err := m.Get(txID)
	if err != nil {
		return err
	}
	if tx == nil || tx.Status != StatusCommitted {
		return fmt.Errorf("transaction not found or not committed")
	}

	rows, err := m.DB.Query(`
		SELECT id, operation, table_name, before_state, committed_state, row_ids
		FROM rest_transaction_operations WHERE transaction_id = $1 ORDER BY id DESC`, txID)
	if err != nil {
		return fmt.Errorf("read operations: %w", err)
	}
	defer rows.Close()

	type rollbackOp struct {
		id             int
		op             string
		tableName      string
		beforeState    []byte
		committedState []byte
		rowIDs         []byte
	}
	var ops []rollbackOp
	for rows.Next() {
		var op rollbackOp
		if err := rows.Scan(&op.id, &op.op, &op.tableName, &op.beforeState, &op.committedState, &op.rowIDs); err != nil {
			return fmt.Errorf("scan operation: %w", err)
		}
		ops = append(ops, op)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate operations: %w", err)
	}

	targetDB, err := m.resolveTargetDB(tx.Schema)
	if err != nil {
		return err
	}

	targetTx, err := targetDB.BeginTx(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("begin target transaction: %w", err)
	}
	defer targetTx.Rollback()

	for _, op := range ops {
		if op.beforeState == nil {
			switch op.op {
			case "INSERT":
				var rowIDs []interface{}
				if err := json.Unmarshal(op.rowIDs, &rowIDs); err != nil {
					return fmt.Errorf("unmarshal row IDs: %w", err)
				}
				for _, id := range rowIDs {
					query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", op.tableName)
					if _, err := targetTx.Exec(query, id); err != nil {
						return fmt.Errorf("rollback INSERT (delete): %w", err)
					}
				}
			}
			continue
		}

		var snapshots []map[string]interface{}
		if err := json.Unmarshal(op.beforeState, &snapshots); err != nil {
			return fmt.Errorf("unmarshal before_state: %w", err)
		}

		switch op.op {
		case "UPDATE":
			if op.committedState != nil {
				var committedSnapshots []map[string]interface{}
				if err := json.Unmarshal(op.committedState, &committedSnapshots); err == nil {
					for _, committed := range committedSnapshots {
						idVal, ok := committed["id"]
						if !ok {
							continue
						}
						checkQuery := fmt.Sprintf("SELECT row_to_json(t) FROM (SELECT * FROM %s WHERE id = $1) t", op.tableName)
						var currentJSON string
						if err := targetTx.QueryRow(checkQuery, idVal).Scan(&currentJSON); err != nil {
							return fmt.Errorf("conflict: cannot read row %v: %w", idVal, err)
						}
						var current map[string]interface{}
						if err := json.Unmarshal([]byte(currentJSON), &current); err != nil {
							return fmt.Errorf("conflict check: %w", err)
						}
						for k, expected := range committed {
							if k == "updated_at" {
								continue
							}
							currentVal, exists := current[k]
							if !exists {
								return fmt.Errorf("conflict: row %v missing column %s", idVal, k)
							}
							if fmt.Sprintf("%v", expected) != fmt.Sprintf("%v", currentVal) {
								return fmt.Errorf("conflict: row %v column %s changed after commit", idVal, k)
							}
						}
					}
				}
			}

			for _, snapshot := range snapshots {
				setClauses := []string{}
				args := []interface{}{}
				i := 1
				for k, v := range snapshot {
					if k == "id" {
						continue
					}
					if k == "updated_at" {
						setClauses = append(setClauses, fmt.Sprintf("%s = $%d", k, i))
						args = append(args, time.Now().UTC())
						i++
						continue
					}
					setClauses = append(setClauses, fmt.Sprintf("%s = $%d", k, i))
					args = append(args, v)
					i++
				}

				if idVal, ok := snapshot["id"]; ok {
					whereIdx := i
					args = append(args, idVal)

					query := fmt.Sprintf("UPDATE %s SET %s WHERE id = $%d",
						op.tableName,
						strings.Join(setClauses, ", "),
						whereIdx)
					if _, err := targetTx.Exec(query, args...); err != nil {
						return fmt.Errorf("rollback UPDATE: %w", err)
					}
				}
			}

		case "DELETE":
			for _, snapshot := range snapshots {
				columns := []string{}
				placeholders := []string{}
				args := []interface{}{}
				i := 1
				for k, v := range snapshot {
					columns = append(columns, k)
					placeholders = append(placeholders, fmt.Sprintf("$%d", i))
					args = append(args, v)
					i++
				}
				query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
					op.tableName,
					strings.Join(columns, ", "),
					strings.Join(placeholders, ", "))
				if _, err := targetTx.Exec(query, args...); err != nil {
					return fmt.Errorf("rollback DELETE (insert): %w", err)
				}
			}
		}
	}

	if err := targetTx.Commit(); err != nil {
		return fmt.Errorf("commit rollback: %w", err)
	}

	now := time.Now().UTC()
	_, err = m.DB.Exec(`UPDATE rest_transactions SET status = $1, updated_at = $2 WHERE id = $3`,
		StatusRolledBack, now, txID)
	if err != nil {
		return fmt.Errorf("mark rolled back: %w", err)
	}

	return nil
}

func (m *Manager) resolveTargetDB(schemaName string) (*sql.DB, error) {
	return m.DB, nil
}

func (m *Manager) StartCleanup(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-m.stopCh:
				return
			case <-ticker.C:
				m.cleanup()
			}
		}
	}()
}

func (m *Manager) StopCleanup() {
	close(m.stopCh)
}

func (m *Manager) cleanup() {
	cutoff := time.Now().UTC().Add(-m.TTL)
	result, err := m.DB.Exec(`
		DELETE FROM rest_transactions
		WHERE (status = $1 AND created_at < $2)
		   OR (status = $3 AND updated_at < $2)`,
		StatusPending, cutoff, StatusCommitted,
	)
	if err != nil {
		slog.Warn("transaction cleanup failed", "error", err)
		return
	}
	count, _ := result.RowsAffected()
	if count > 0 {
		slog.Info("transaction cleanup", "removed", count)
	}
}

func BuildInsertQuery(tableName string, columns []string, placeholders []string) string {
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		tableName,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)
}

func BuildUpdateQuery(tableName string, setClauses []string, placeholders []string) string {
	return fmt.Sprintf("UPDATE %s SET %s WHERE id = $%d",
		tableName,
		strings.Join(setClauses, ", "),
		len(setClauses)+1,
	)
}

func BuildDeleteQuery(tableName string) string {
	return fmt.Sprintf("DELETE FROM %s WHERE id = $1", tableName)
}
