package response

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func WriteJSON(c *gin.Context, rows *sql.Rows, columns []string, totalCount *int) {
	c.Header("Content-Type", "application/json")
	c.Header("Content-Profile", c.GetString("schema"))

	var results []map[string]interface{}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    "PGRST100",
				"message": "Failed to scan row",
				"details": err.Error(),
			})
			return
		}

		row := make(map[string]interface{}, len(columns))
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				var jsonVal interface{}
				if err := json.Unmarshal(b, &jsonVal); err == nil {
					row[col] = jsonVal
				} else {
					row[col] = string(b)
				}
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	c.JSON(http.StatusOK, results)
}

func WriteSingularJSON(c *gin.Context, row map[string]interface{}) {
	c.Header("Content-Type", "application/vnd.pgrst.object+json")
	c.JSON(http.StatusOK, row)
}

func WriteCSV(c *gin.Context, rows *sql.Rows, columns []string) {
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.csv", c.Param("table")))

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	writer.Write(columns)

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return
		}

		record := make([]string, len(columns))
		for i, val := range values {
			if val == nil {
				record[i] = ""
			} else if b, ok := val.([]byte); ok {
				record[i] = string(b)
			} else {
				record[i] = fmt.Sprintf("%v", val)
			}
		}
		writer.Write(record)
	}
}

func WriteInserted(c *gin.Context, count int, prefer []string) {
	if containsPrefer(prefer, "return=representation") {
		c.Header("Content-Type", "application/json")
		c.JSON(http.StatusCreated, gin.H{
			"message": fmt.Sprintf("%d row(s) inserted", count),
		})
		return
	}

	c.Status(http.StatusCreated)
}

func WriteUpdated(c *gin.Context, count int, prefer []string) {
	if containsPrefer(prefer, "return=representation") {
		c.Header("Content-Type", "application/json")
		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("%d row(s) updated", count),
		})
		return
	}

	c.Status(http.StatusNoContent)
}

func WriteDeleted(c *gin.Context, count int, prefer []string) {
	if containsPrefer(prefer, "return=representation") {
		c.Header("Content-Type", "application/json")
		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("%d row(s) deleted", count),
		})
		return
	}

	c.Status(http.StatusNoContent)
}

func SetContentRange(c *gin.Context, from, to, total int) {
	c.Header("Content-Range", fmt.Sprintf("%d-%d/%d", from, to, total))
	c.Header("Range-Unit", "items")

	if total > 0 {
		c.Header("X-Total-Count", strconv.Itoa(total))
	}
}

func SetRangeHeaders(c *gin.Context, limit, offset, total int) {
	if limit >= 0 {
		first := offset
		last := offset + limit - 1
		if total > 0 && last >= total {
			last = total - 1
		}
		SetContentRange(c, first, last, total)
	}
}

func containsPrefer(prefer []string, value string) bool {
	for _, p := range prefer {
		if strings.Contains(p, value) {
			return true
		}
	}
	return false
}

func ParseRangeHeader(r *http.Request) (limit, offset int, ok bool) {
	rangeHeader := r.Header.Get("Range")
	if rangeHeader == "" {
		return -1, 0, false
	}

	parts := strings.SplitN(rangeHeader, "=", 2)
	if len(parts) != 2 || parts[0] != "items" {
		return -1, 0, false
	}

	bounds := strings.Split(parts[1], "-")
	if len(bounds) != 2 {
		return -1, 0, false
	}

	start, err := strconv.Atoi(bounds[0])
	if err != nil {
		return -1, 0, false
	}

	end, err := strconv.Atoi(bounds[1])
	if err != nil {
		return -1, 0, false
	}

	if start > end {
		return -1, 0, false
	}

	limit = end - start + 1
	offset = start

	return limit, offset, true
}

func WriteCSVWriter(w io.Writer, rows *sql.Rows, columns []string) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	writer.Write(columns)

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return err
		}

		record := make([]string, len(columns))
		for i, val := range values {
			if val == nil {
				record[i] = ""
			} else if b, ok := val.([]byte); ok {
				record[i] = string(b)
			} else {
				record[i] = fmt.Sprintf("%v", val)
			}
		}
		writer.Write(record)
	}

	return rows.Err()
}

func CalculateRange(limit, offset, total int) (int, int) {
	if limit < 0 {
		return 0, total
	}

	from := offset
	to := offset + limit

	if total > 0 && to > total {
		to = total
	}

	if from > total {
		return total, total
	}

	return from, to
}

func MustParseInt(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}

func MaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func CeilDiv(a, b int) int {
	return int(math.Ceil(float64(a) / float64(b)))
}
