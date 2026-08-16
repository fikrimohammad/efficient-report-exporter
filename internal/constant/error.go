package constant

import (
	"errors"
	"net/http"

	sdkerrs "github.com/fikrimohammad/go-dev-sdk/errs/v2"
)

// Error codes. These are plain integers owned by the application; the v2 errs
// SDK no longer defines a code taxonomy.
const (
	OK              = 0
	InvalidArgument = 1001
	Conflict        = 1002
	NotFound        = 4004
	Internal        = 5001
	DBInternal      = 5002
	CacheInternal   = 5003
	MQInternal      = 5004
	S3Internal      = 5005
)

// CodeFromError extracts the code from an error chain. It returns Internal when
// the error is not an SDK error.
func CodeFromError(err error) int {
	if err == nil {
		return OK
	}

	var se *sdkerrs.Error
	if errors.As(err, &se) {
		return se.Code()
	}

	return Internal
}

// HTTPStatus returns the HTTP status code for an error code, accounting for the
// application-level Conflict code that maps to 409.
func HTTPStatus(code int) int {
	switch {
	case code == OK:
		return http.StatusOK
	case code == Conflict:
		return http.StatusConflict
	case code >= 1000 && code < 4000:
		return http.StatusBadRequest
	case code >= 4000 && code < 5000:
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
