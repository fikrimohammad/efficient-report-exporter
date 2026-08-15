package report

import (
	"testing"

	"github.com/fikrimohammad/efficient-report-exporter/internal/mocks"
	"go.uber.org/mock/gomock"
)

func newMockRepositories(t *testing.T) (*mocks.MockMySQL, *mocks.MockMQ, *mocks.MockRedis, *mocks.MockS3) {
	t.Helper()
	ctrl := gomock.NewController(t)
	return mocks.NewMockMySQL(ctrl), mocks.NewMockMQ(ctrl), mocks.NewMockRedis(ctrl), mocks.NewMockS3(ctrl)
}

func TestNew_Success(t *testing.T) {
	mysql, mq, redis, s3 := newMockRepositories(t)
	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	uc, err := New(mysql, mq, redis, s3, dl)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if uc == nil {
		t.Fatal("expected non-nil use case")
	}
}

func TestNew_NilMySQL(t *testing.T) {
	_, mq, redis, s3 := newMockRepositories(t)
	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	_, err := New(nil, mq, redis, s3, dl)
	if err == nil {
		t.Fatal("expected error for nil mysql repository")
	}
}

func TestNew_NilMQ(t *testing.T) {
	mysql, _, redis, s3 := newMockRepositories(t)
	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	_, err := New(mysql, nil, redis, s3, dl)
	if err == nil {
		t.Fatal("expected error for nil mq repository")
	}
}

func TestNew_NilRedis(t *testing.T) {
	mysql, mq, _, s3 := newMockRepositories(t)
	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	_, err := New(mysql, mq, nil, s3, dl)
	if err == nil {
		t.Fatal("expected error for nil redis repository")
	}
}

func TestNew_NilS3(t *testing.T) {
	mysql, mq, redis, _ := newMockRepositories(t)
	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	_, err := New(mysql, mq, redis, nil, dl)
	if err == nil {
		t.Fatal("expected error for nil s3 repository")
	}
}
