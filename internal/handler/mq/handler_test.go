package mq

import (
	"errors"
	"testing"

	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
	"github.com/fikrimohammad/go-dev-sdk/errs/v2"
)

func TestNewHandler_NilUseCase(t *testing.T) {
	_, err := New(nil)
	if err == nil {
		t.Fatal("expected error for nil use case")
	}

	var e *errs.Error
	if !errors.As(err, &e) {
		t.Fatalf("expected *errs.Error, got %T", err)
	}
	if e.Code() != constant.Internal {
		t.Errorf("expected code Internal, got %v", e.Code())
	}
}
