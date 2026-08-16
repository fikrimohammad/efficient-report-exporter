package s3

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/fikrimohammad/efficient-report-exporter/internal/mock"
	"github.com/fikrimohammad/efficient-report-exporter/internal/repository"
	commons3 "github.com/fikrimohammad/go-dev-sdk/s3"
	"go.uber.org/mock/gomock"
)

func TestUploadReportFileUsesClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	cli := mock.NewMockS3Client(ctrl)

	var captured commons3.UploadObjectParams
	cli.EXPECT().
		UploadObject(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, p commons3.UploadObjectParams) error {
			captured = p
			return nil
		})

	repo := &repo{s3: cli}

	payload := strings.Repeat("a", 10)
	err := repo.UploadReportFile(context.Background(), repository.UploadReportFileParams{
		FileName: "test.csv",
		FileData: io.NopCloser(strings.NewReader(payload)),
	})
	if err != nil {
		t.Fatalf("UploadReportFile returned an error: %v", err)
	}

	if captured.Bucket != "reports" {
		t.Fatalf("bucket = %q, want reports", captured.Bucket)
	}
	if captured.Key != "test.csv" {
		t.Fatalf("key = %q, want test.csv", captured.Key)
	}
	if captured.ContentType != "application/octet-stream" {
		t.Fatalf("content type = %q, want application/octet-stream", captured.ContentType)
	}
	if captured.Body == nil {
		t.Fatal("expected a non-nil body")
	}
}

func TestUploadReportFile_SkipsClientOnMissingFileData(t *testing.T) {
	cli := mock.NewMockS3Client(gomock.NewController(t))
	repo := &repo{s3: cli}

	err := repo.UploadReportFile(context.Background(), repository.UploadReportFileParams{
		FileName: "test.csv",
	})
	if err == nil {
		t.Fatal("expected error for missing file data")
	}
}

func TestUploadReportFile_SkipsClientOnMissingFileName(t *testing.T) {
	cli := mock.NewMockS3Client(gomock.NewController(t))
	repo := &repo{s3: cli}

	err := repo.UploadReportFile(context.Background(), repository.UploadReportFileParams{
		FileData: io.NopCloser(strings.NewReader("x")),
	})
	if err == nil {
		t.Fatal("expected error for missing file name")
	}
}

func TestUploadReportFile_UploadError(t *testing.T) {
	ctrl := gomock.NewController(t)
	cli := mock.NewMockS3Client(ctrl)
	cli.EXPECT().
		UploadObject(gomock.Any(), gomock.Any()).
		Return(errors.New("upload failed"))

	repo := &repo{s3: cli}

	err := repo.UploadReportFile(context.Background(), repository.UploadReportFileParams{
		FileName: "test.csv",
		FileData: io.NopCloser(strings.NewReader("x")),
	})
	if err == nil {
		t.Fatal("expected error from uploader")
	}
}
