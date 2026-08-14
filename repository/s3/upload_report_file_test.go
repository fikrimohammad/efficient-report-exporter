package s3

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/fikrimohammad/efficient-report-exporter/repository"
)

func TestUploadReportFileUsesClient(t *testing.T) {
	cli := &stubClient{}
	repo := &repo{s3: cli}

	payload := strings.Repeat("a", 10)
	err := repo.UploadReportFile(context.Background(), repository.UploadReportFileParams{
		FileName: "test.csv",
		FileData: io.NopCloser(strings.NewReader(payload)),
	})
	if err != nil {
		t.Fatalf("UploadReportFile returned an error: %v", err)
	}

	if cli.uploadCalls != 1 {
		t.Fatalf("expected 1 upload call, got %d", cli.uploadCalls)
	}
	if cli.lastUpload.Bucket != "reports" {
		t.Fatalf("bucket = %q, want reports", cli.lastUpload.Bucket)
	}
	if cli.lastUpload.Key != "test.csv" {
		t.Fatalf("key = %q, want test.csv", cli.lastUpload.Key)
	}
	if cli.lastUpload.ContentType != "application/octet-stream" {
		t.Fatalf("content type = %q, want application/octet-stream", cli.lastUpload.ContentType)
	}
	if cli.lastUpload.Body == nil {
		t.Fatal("expected a non-nil body")
	}
}

func TestUploadReportFile_SkipsClientOnMissingFileData(t *testing.T) {
	cli := &stubClient{}
	repo := &repo{s3: cli}

	err := repo.UploadReportFile(context.Background(), repository.UploadReportFileParams{
		FileName: "test.csv",
	})
	if err == nil {
		t.Fatal("expected error for missing file data")
	}
	if cli.uploadCalls != 0 {
		t.Fatalf("expected 0 upload calls, got %d", cli.uploadCalls)
	}
}

func TestUploadReportFile_SkipsClientOnMissingFileName(t *testing.T) {
	cli := &stubClient{}
	repo := &repo{s3: cli}

	err := repo.UploadReportFile(context.Background(), repository.UploadReportFileParams{
		FileData: io.NopCloser(strings.NewReader("x")),
	})
	if err == nil {
		t.Fatal("expected error for missing file name")
	}
	if cli.uploadCalls != 0 {
		t.Fatalf("expected 0 upload calls, got %d", cli.uploadCalls)
	}
}

func TestUploadReportFile_UploadError(t *testing.T) {
	cli := &stubClient{uploadErr: errUpload}
	repo := &repo{s3: cli}

	err := repo.UploadReportFile(context.Background(), repository.UploadReportFileParams{
		FileName: "test.csv",
		FileData: io.NopCloser(strings.NewReader("x")),
	})
	if err == nil {
		t.Fatal("expected error from uploader")
	}
}
