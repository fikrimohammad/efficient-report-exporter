package s3

import (
	"context"
	"testing"

	"github.com/fikrimohammad/efficient-report-exporter/repository"
)

func TestGeneratePresignedDownloadURL_Success(t *testing.T) {
	cli := &stubClient{presignURL: "https://s3.example.com/reports/test.csv?X-Amz-Signature=abc"}
	repo := &repo{s3: cli}

	url, err := repo.GeneratePresignedDownloadURL(context.Background(), repository.GeneratePresignedDownloadURLParams{
		FileName: "test.csv",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if url != "https://s3.example.com/reports/test.csv?X-Amz-Signature=abc" {
		t.Fatalf("url = %q", url)
	}
	if cli.presignCalls != 1 {
		t.Fatalf("expected 1 presign call, got %d", cli.presignCalls)
	}
	if cli.lastPresign.Bucket != "reports" {
		t.Fatalf("bucket = %q, want reports", cli.lastPresign.Bucket)
	}
	if cli.lastPresign.Key != "test.csv" {
		t.Fatalf("key = %q, want test.csv", cli.lastPresign.Key)
	}
	if cli.lastPresign.ExpiresIn != 0 {
		t.Fatalf("expires in = %v, want 0 (client default applies)", cli.lastPresign.ExpiresIn)
	}
}

func TestGeneratePresignedDownloadURL_EmptyFileName(t *testing.T) {
	cli := &stubClient{}
	repo := &repo{s3: cli}

	_, err := repo.GeneratePresignedDownloadURL(context.Background(), repository.GeneratePresignedDownloadURLParams{
		FileName: "",
	})
	if err == nil {
		t.Fatal("expected error for empty file name")
	}
	if cli.presignCalls != 0 {
		t.Fatalf("expected 0 presign calls, got %d", cli.presignCalls)
	}
}

func TestGeneratePresignedDownloadURL_SignError(t *testing.T) {
	cli := &stubClient{presignErr: errPresign, presignURL: "https://s3.example.com/reports/test.csv"}
	repo := &repo{s3: cli}

	_, err := repo.GeneratePresignedDownloadURL(context.Background(), repository.GeneratePresignedDownloadURLParams{
		FileName: "test.csv",
	})
	if err == nil {
		t.Fatal("expected error from presigner")
	}
}
