package mq

import (
	"errors"
	"testing"

	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
	"github.com/fikrimohammad/go-dev-sdk/errs/v2"
)

func TestNew_Success(t *testing.T) {
	_, err := New(newMockProducerClient(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNew_NilProducer(t *testing.T) {
	_, err := New(nil)
	if err == nil {
		t.Fatal("expected error for nil producer")
	}

	var e *errs.Error
	if !errors.As(err, &e) {
		t.Fatalf("expected *errs.Error, got %T", err)
	}
	if e.Code() != constant.Internal {
		t.Errorf("expected code Internal, got %v", e.Code())
	}
}
