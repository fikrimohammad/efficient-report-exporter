package s3

import (
	"context"
	"errors"
	"testing"

	"github.com/fikrimohammad/efficient-report-exporter/internal/mocks"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
	commons3 "github.com/fikrimohammad/go-dev-sdk/s3"
	"go.uber.org/mock/gomock"
)

func TestGeneratePresignedDownloadURL_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	cli := mocks.NewMockS3Client(ctrl)

	var captured commons3.PresignGetObjectParams
	cli.EXPECT().
		PresignGetObject(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, p commons3.PresignGetObjectParams) (string, error) {
			captured = p
			return "https://s3.example.com/reports/test.csv?X-Amz-Signature=abc", nil
		})

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
	if captured.Bucket != "reports" {
		t.Fatalf("bucket = %q, want reports", captured.Bucket)
	}
	if captured.Key != "test.csv" {
		t.Fatalf("key = %q, want test.csv", captured.Key)
	}
	if captured.ExpiresIn != 0 {
		t.Fatalf("expires in = %v, want 0 (client default applies)", captured.ExpiresIn)
	}
}

func TestGeneratePresignedDownloadURL_EmptyFileName(t *testing.T) {
	cli := mocks.NewMockS3Client(gomock.NewController(t))
	repo := &repo{s3: cli}

	_, err := repo.GeneratePresignedDownloadURL(context.Background(), repository.GeneratePresignedDownloadURLParams{
		FileName: "",
	})
	if err == nil {
		t.Fatal("expected error for empty file name")
	}
}

func TestGeneratePresignedDownloadURL_SignError(t *testing.T) {
	ctrl := gomock.NewController(t)
	cli := mocks.NewMockS3Client(ctrl)
	cli.EXPECT().
		PresignGetObject(gomock.Any(), gomock.Any()).
		Return("", errors.New("presign failed"))

	repo := &repo{s3: cli}

	_, err := repo.GeneratePresignedDownloadURL(context.Background(), repository.GeneratePresignedDownloadURLParams{
		FileName: "test.csv",
	})
	if err == nil {
		t.Fatal("expected error from presigner")
	}
}
