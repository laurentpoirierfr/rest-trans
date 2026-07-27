package transaction

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	Manager *Manager
}

func NewHandler(manager *Manager) *Handler {
	return &Handler{Manager: manager}
}

func (h *Handler) Start(c *gin.Context) {
	schemaName := c.Param("schema")
	if schemaName == "" {
		schemaName = "public"
	}

	tx, err := h.Manager.Start(schemaName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "PGRST300",
			"message": fmt.Sprintf("Failed to start transaction: %v", err),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"tx": tx.ID,
	})
}

func (h *Handler) List(c *gin.Context) {
	schemaName := c.Param("schema")
	if schemaName == "" {
		schemaName = "public"
	}

	txs, err := h.Manager.List(schemaName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "PGRST301",
			"message": fmt.Sprintf("Failed to list transactions: %v", err),
		})
		return
	}

	if txs == nil {
		txs = []Transaction{}
	}

	c.JSON(http.StatusOK, txs)
}

func (h *Handler) Get(c *gin.Context) {
	txID := c.Param("txID")

	tx, err := h.Manager.Get(txID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "PGRST302",
			"message": fmt.Sprintf("Failed to get transaction: %v", err),
		})
		return
	}

	if tx == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    "PGRST303",
			"message": fmt.Sprintf("Transaction not found: %s", txID),
		})
		return
	}

	c.JSON(http.StatusOK, tx)
}

func (h *Handler) Commit(c *gin.Context) {
	txID := c.Param("txID")

	tx, err := h.Manager.Get(txID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "PGRST304",
			"message": fmt.Sprintf("Failed to get transaction: %v", err),
		})
		return
	}

	if tx == nil || tx.Status != StatusPending {
		c.JSON(http.StatusConflict, gin.H{
			"code":    "PGRST305",
			"message": "Transaction not found or not pending",
		})
		return
	}

	if err := h.Manager.Commit(txID); err != nil {
		c.JSON(http.StatusConflict, gin.H{
			"code":    "PGRST306",
			"message": fmt.Sprintf("Failed to commit transaction: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, CommitResponse{Status: "committed"})
}

func (h *Handler) Rollback(c *gin.Context) {
	txID := c.Param("txID")

	tx, err := h.Manager.Get(txID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "PGRST307",
			"message": fmt.Sprintf("Failed to get transaction: %v", err),
		})
		return
	}

	if tx == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    "PGRST303",
			"message": fmt.Sprintf("Transaction not found: %s", txID),
		})
		return
	}

	if tx.Status == StatusPending {
		if err := h.Manager.Rollback(txID); err != nil {
			c.JSON(http.StatusConflict, gin.H{
				"code":    "PGRST309",
				"message": fmt.Sprintf("Failed to rollback transaction: %v", err),
			})
			return
		}
		c.JSON(http.StatusOK, RollbackResponse{Status: "rolled_back"})
		return
	}

	if tx.Status == StatusCommitted {
		if err := h.Manager.RollbackPostCommit(txID); err != nil {
			c.JSON(http.StatusConflict, gin.H{
				"code":    "PGRST320",
				"message": fmt.Sprintf("Failed to rollback committed transaction: %v", err),
			})
			return
		}
		c.JSON(http.StatusOK, RollbackResponse{Status: "rolled_back"})
		return
	}

	c.JSON(http.StatusConflict, gin.H{
		"code":    "PGRST308",
		"message": fmt.Sprintf("Transaction cannot be rolled back: status=%s", tx.Status),
	})
}
