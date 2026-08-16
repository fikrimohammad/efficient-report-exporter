package mq

import (
	"context"
	"errors"
	"testing"

	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
	mqmodel "github.com/fikrimohammad/efficient-report-exporter/internal/model/mq"
	"github.com/fikrimohammad/go-dev-sdk/errs/v2"
	"go.uber.org/mock/gomock"
)

func TestPublishExportReportProcessMsg_Success(t *testing.T) {
	producer := newMockProducerClient(t)

	var topic, tag, key string
	var message []byte
	producer.EXPECT().
		PublishSync(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, tpc, tg, k string, m []byte) error {
			topic, tag, key, message = tpc, tg, k, m
			return nil
		})

	repo, err := New(producer)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	err = repo.PublishExportReportProcessMsg(context.Background(), mqmodel.ExportReportProcessMessage{
		JobID: "42",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if topic != "reporting" {
		t.Errorf("expected topic 'reporting', got %s", topic)
	}
	if tag != "export_report_process" {
		t.Errorf("expected tag 'export_report_process', got %s", tag)
	}
	if key != "42" {
		t.Errorf("expected key '42', got %s", key)
	}
	if len(message) == 0 {
		t.Error("expected non-empty message")
	}
}

func TestPublishExportReportProcessMsg_PublishError(t *testing.T) {
	producer := newMockProducerClient(t)
	producer.EXPECT().
		PublishSync(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("publish failed"))

	repo, err := New(producer)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	err = repo.PublishExportReportProcessMsg(context.Background(), mqmodel.ExportReportProcessMessage{
		JobID: "42",
	})
	if err == nil {
		t.Fatal("expected error")
	}

	var e *errs.Error
	if !errors.As(err, &e) {
		t.Fatalf("expected *errs.Error, got %T", err)
	}
	if e.Code() != constant.MQInternal {
		t.Errorf("expected code MQInternal, got %v", e.Code())
	}
}
