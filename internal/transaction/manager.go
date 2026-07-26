package transaction

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
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

func (m *Manager) Stage(txID, op, tableName, sqlQuery string, params []interface{}) error {
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
		INSERT INTO rest_transaction_operations (transaction_id, operation, table_name, sql_query, params)
		VALUES ($1, $2, $3, $4, $5)`,
		txID, op, tableName, sqlQuery, paramsJSON,
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
		SELECT id, operation, table_name, sql_query, COALESCE(params, '[]'::jsonb)
		FROM rest_transaction_operations WHERE transaction_id = $1 ORDER BY id`, txID)
	if err != nil {
		return fmt.Errorf("read operations: %w", err)
	}

	type stagedOp struct {
		id        int
		op        string
		tableName string
		sqlQuery  string
		params    string
	}
	var ops []stagedOp
	for rows.Next() {
		var op stagedOp
		if err := rows.Scan(&op.id, &op.op, &op.tableName, &op.sqlQuery, &op.params); err != nil {
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
		if len(params) > 0 {
			_, err = targetTx.Exec(op.sqlQuery, params...)
		} else {
			_, err = targetTx.Exec(op.sqlQuery)
		}
		if err != nil {
			return fmt.Errorf("execute op %d (%s): %w", op.id, op.op, err)
		}
	}

	if err := targetTx.Commit(); err != nil {
		return fmt.Errorf("commit target: %w", err)
	}

	now := time.Now().UTC()
	_, err = lockTx.Exec(`DELETE FROM rest_transaction_operations WHERE transaction_id = $1`, txID)
	if err != nil {
		return fmt.Errorf("cleanup operations: %w", err)
	}

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
		WHERE status = $1 AND created_at < $2`, StatusPending, cutoff,
	)
	if err != nil {
		log.Printf("Warning: transaction cleanup failed: %v", err)
		return
	}
	count, _ := result.RowsAffected()
	if count > 0 {
		log.Printf("Transaction cleanup: removed %d expired transactions", count)
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
