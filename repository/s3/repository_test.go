package s3

import (
	"testing"
)

func TestNew_NilClient(t *testing.T) {
	_, err := New(nil)
	if err == nil {
		t.Fatal("expected error for nil s3 client")
	}
}
