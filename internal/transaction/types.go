package transaction

import "time"

type Status string

const (
	StatusPending   Status = "pending"
	StatusCommitted Status = "committed"
	StatusRolledBack Status = "rolled_back"
)

type Transaction struct {
	ID        string    `json:"id"`
	Schema    string    `json:"schema"`
	Status    Status    `json:"status"`
	OpCount   int       `json:"operation_count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Operation struct {
	ID            int       `json:"id"`
	TransactionID string    `json:"transaction_id"`
	Operation     string    `json:"operation"`
	TableName     string    `json:"table_name"`
	SQLQuery      string    `json:"sql_query"`
	Params        []byte    `json:"params,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type StagedResponse struct {
	Status string `json:"status"`
	Tx     string `json:"tx"`
}

type CommitResponse struct {
	Status string `json:"status"`
}

type RollbackResponse struct {
	Status string `json:"status"`
}
