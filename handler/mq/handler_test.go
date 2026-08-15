package mq

import (
	"testing"

	"github.com/fikrimohammad/efficient-report-exporter/apperrors"
	"github.com/fikrimohammad/go-dev-sdk/errs"
)

func TestNewHandler_NilUseCase(t *testing.T) {
	_, err := New(nil)
	if err == nil {
		t.Fatal("expected error for nil use case")
	}

	var e *errs.Error
	if !errs.As(err, &e) {
		t.Fatalf("expected *errs.Error, got %T", err)
	}
	if e.Code() != apperrors.Internal {
		t.Errorf("expected code Internal, got %v", e.Code())
	}
}
