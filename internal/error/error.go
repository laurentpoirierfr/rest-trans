package apierror

import (
	"fmt"
	"net/http"
)

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
	Hint    string `json:"hint,omitempty"`
	Status  int    `json:"-"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *Error) HTTPStatus() int {
	return e.Status
}

var (
	ErrInvalidFilter = func(detail string) *Error {
		return &Error{
			Code:    "PGRST100",
			Message: "Invalid filter syntax",
			Details: detail,
			Status:  http.StatusBadRequest,
		}
	}

	ErrInvalidColumn = func(col string) *Error {
		return &Error{
			Code:    "PGRST204",
			Message: fmt.Sprintf("Column '%s' not found in the table", col),
			Status:  http.StatusBadRequest,
		}
	}

	ErrInvalidOperator = func(op string) *Error {
		return &Error{
			Code:    "PGRST106",
			Message: fmt.Sprintf("Invalid operator '%s'", op),
			Details: "Supported operators: eq, neq, gt, gte, lt, lte, like, ilike, in, is, match, imatch",
			Status:  http.StatusBadRequest,
		}
	}

	ErrTableNotFound = func(name string) *Error {
		return &Error{
			Code:    "PGRST205",
			Message: fmt.Sprintf("Table '%s' not found", name),
			Status:  http.StatusNotFound,
		}
	}

	ErrMethodNotAllowed = func(method string) *Error {
		return &Error{
			Code:    "PGRST201",
			Message: fmt.Sprintf("Method '%s' not allowed", method),
			Status:  http.StatusMethodNotAllowed,
		}
	}

	ErrInvalidBody = func(detail string) *Error {
		return &Error{
			Code:    "PGRST200",
			Message: "Invalid request body",
			Details: detail,
			Status:  http.StatusUnprocessableEntity,
		}
	}

	ErrInvalidRange = func(detail string) *Error {
		return &Error{
			Code:    "PGRST106",
			Message: "Invalid Range header",
			Details: detail,
			Status: http.StatusRequestedRangeNotSatisfiable,
		}
	}

	ErrNoOverlap = func() *Error {
		return &Error{
			Code:    "PGRST106",
			Message: "Range start is greater than the total count",
			Status:  http.StatusRequestedRangeNotSatisfiable,
		}
	}

	ErrDatabase = func(detail string) *Error {
		return &Error{
			Code:    "PGRST100",
			Message: "Database error",
			Details: detail,
			Status:  http.StatusInternalServerError,
		}
	}

	ErrConflict = func(detail string) *Error {
		return &Error{
			Code:    "23505",
			Message: "Unique constraint violation",
			Details: detail,
			Status:  http.StatusConflict,
		}
	}

	ErrInvalidOrder = func(detail string) *Error {
		return &Error{
			Code:    "PGRST100",
			Message: "Invalid order syntax",
			Details: detail,
			Status:  http.StatusBadRequest,
		}
	}

	ErrSingularReturn = func() *Error {
		return &Error{
			Code:    "PGRST106",
			Message: "Multiple rows returned but singular response requested",
			Status:  http.StatusMultipleChoices,
		}
	}
)
