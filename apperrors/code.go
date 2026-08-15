// Package apperrors defines the application's error codes. Code is an alias of
// the shared SDK's code type, so application-defined codes (such as Conflict)
// are interchangeable with SDK codes when constructing errors via
// errs.New/errs.Wrap.
package apperrors

import (
	"errors"
	"net/http"

	sdkerrs "github.com/fikrimohammad/go-dev-sdk/errs"
)

// Code is an alias of the SDK's error-code type.
type Code = sdkerrs.Code

const (
	OK              = sdkerrs.OK
	InvalidArgument = sdkerrs.InvalidArgument
	Conflict        = sdkerrs.Code(1002)
	NotFound        = sdkerrs.NotFound
	Internal        = sdkerrs.Internal
	DBInternal      = sdkerrs.DBInternal
	CacheInternal   = sdkerrs.CacheInternal
	MQInternal      = sdkerrs.MQInternal
	S3Internal      = sdkerrs.S3Internal
)

// CodeFromError extracts the Code from an error chain. It returns Internal when
// the error is not an SDK error.
func CodeFromError(err error) Code {
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
// application-level Conflict code that the SDK does not map.
func HTTPStatus(code Code) int {
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
