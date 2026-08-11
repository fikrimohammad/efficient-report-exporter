package report

import (
	"testing"
)

func TestNew_Success(t *testing.T) {
	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	uc, err := New(defaultMockMySQL(), defaultMockMQ(), defaultMockRedis(), defaultMockS3(), dl)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if uc == nil {
		t.Fatal("expected non-nil use case")
	}
}

func TestNew_NilMySQL(t *testing.T) {
	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	_, err := New(nil, defaultMockMQ(), defaultMockRedis(), defaultMockS3(), dl)
	if err == nil {
		t.Fatal("expected error for nil mysql repository")
	}
}

func TestNew_NilMQ(t *testing.T) {
	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	_, err := New(defaultMockMySQL(), nil, defaultMockRedis(), defaultMockS3(), dl)
	if err == nil {
		t.Fatal("expected error for nil mq repository")
	}
}

func TestNew_NilRedis(t *testing.T) {
	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	_, err := New(defaultMockMySQL(), defaultMockMQ(), nil, defaultMockS3(), dl)
	if err == nil {
		t.Fatal("expected error for nil redis repository")
	}
}

func TestNew_NilS3(t *testing.T) {
	dl := newTestDynamicLoader(t)
	defer func() { _ = dl.Stop() }()

	_, err := New(defaultMockMySQL(), defaultMockMQ(), defaultMockRedis(), nil, dl)
	if err == nil {
		t.Fatal("expected error for nil s3 repository")
	}
}
