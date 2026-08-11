package mq

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/fikrimohammad/efficient-report-exporter/model"
	"github.com/fikrimohammad/go-dev-sdk/errs"
)

type stubProducer struct {
	publishSyncCalls        []publishSyncCall
	publishSyncErr          error
	publishSyncWithDelayErr error
}

type publishSyncCall struct {
	topic string
	tag   string
	key   string
	msg   []byte
}

func (s *stubProducer) PublishSync(_ context.Context, topic, tag, key string, message []byte) error {
	s.publishSyncCalls = append(s.publishSyncCalls, publishSyncCall{topic: topic, tag: tag, key: key, msg: message})
	return s.publishSyncErr
}

func (s *stubProducer) PublishSyncWithDelay(_ context.Context, topic, tag, key string, message []byte, _ time.Duration) error {
	s.publishSyncCalls = append(s.publishSyncCalls, publishSyncCall{topic: topic, tag: tag, key: key, msg: message})
	return s.publishSyncWithDelayErr
}

func (s *stubProducer) PublishAsync(_ context.Context, topic, tag, key string, message []byte,
	callback func(ctx context.Context, result *primitive.SendResult, err error)) error {
	return nil
}

func TestNew_Success(t *testing.T) {
	producer := &stubProducer{}
	_, err := New(producer)
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
	if !errs.As(err, &e) {
		t.Fatalf("expected *errs.Error, got %T", err)
	}
	if e.Code() != errs.Internal {
		t.Errorf("expected code Internal, got %v", e.Code())
	}
}

func TestPublishExportReportProcessMsg_Success(t *testing.T) {
	producer := &stubProducer{}
	repo, err := New(producer)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	err = repo.PublishExportReportProcessMsg(context.Background(), model.ExportReportProcessMessage{
		JobID: 42,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(producer.publishSyncCalls) != 1 {
		t.Fatalf("expected 1 publish call, got %d", len(producer.publishSyncCalls))
	}

	call := producer.publishSyncCalls[0]
	if call.topic != "reporting" {
		t.Errorf("expected topic 'reporting', got %s", call.topic)
	}
	if call.tag != "export_report_process" {
		t.Errorf("expected tag 'export_report_process', got %s", call.tag)
	}
	if call.key != "42" {
		t.Errorf("expected key '42', got %s", call.key)
	}
	if len(call.msg) == 0 {
		t.Error("expected non-empty message")
	}
}

func TestPublishExportReportProcessMsg_PublishError(t *testing.T) {
	producer := &stubProducer{publishSyncErr: errors.New("publish failed")}
	repo, err := New(producer)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	err = repo.PublishExportReportProcessMsg(context.Background(), model.ExportReportProcessMessage{
		JobID: 42,
	})
	if err == nil {
		t.Fatal("expected error")
	}

	var e *errs.Error
	if !errs.As(err, &e) {
		t.Fatalf("expected *errs.Error, got %T", err)
	}
	if e.Code() != errs.MQInternal {
		t.Errorf("expected code MQInternal, got %v", e.Code())
	}
}

func TestPublishExportReportDoneMsg_Success(t *testing.T) {
	producer := &stubProducer{}
	repo, err := New(producer)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	err = repo.PublishExportReportDoneMsg(context.Background(), model.ExportReportDoneMessage{
		JobID: 99,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(producer.publishSyncCalls) != 1 {
		t.Fatalf("expected 1 publish call, got %d", len(producer.publishSyncCalls))
	}

	call := producer.publishSyncCalls[0]
	if call.topic != "reporting" {
		t.Errorf("expected topic 'reporting', got %s", call.topic)
	}
	if call.tag != "export_report_done" {
		t.Errorf("expected tag 'export_report_done', got %s", call.tag)
	}
	if call.key != "99" {
		t.Errorf("expected key '99', got %s", call.key)
	}
	if len(call.msg) == 0 {
		t.Error("expected non-empty message")
	}
}

func TestPublishExportReportDoneMsg_PublishError(t *testing.T) {
	producer := &stubProducer{publishSyncErr: errors.New("publish failed")}
	repo, err := New(producer)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	err = repo.PublishExportReportDoneMsg(context.Background(), model.ExportReportDoneMessage{
		JobID: 99,
	})
	if err == nil {
		t.Fatal("expected error")
	}

	var e *errs.Error
	if !errs.As(err, &e) {
		t.Fatalf("expected *errs.Error, got %T", err)
	}
	if e.Code() != errs.MQInternal {
		t.Errorf("expected code MQInternal, got %v", e.Code())
	}
}
