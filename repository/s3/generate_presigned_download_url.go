package s3

import (
	"context"
	"fmt"

	"github.com/fikrimohammad/efficient-report-exporter/common/errs"
	commons3 "github.com/fikrimohammad/efficient-report-exporter/common/s3"
	"github.com/fikrimohammad/efficient-report-exporter/constant"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
)

func (r *repo) GeneratePresignedDownloadURL(ctx context.Context, params repository.GeneratePresignedDownloadURLParams) (string, error) {
	if params.FileName == "" {
		return "", errs.New(errs.InvalidArgument, "file_name is required")
	}

	presignURL, err := r.s3.PresignGetObject(ctx, commons3.PresignGetObjectParams{
		Bucket:                     constant.ReportFileBucket,
		Key:                        params.FileName,
		ResponseContentType:        constant.ContentTypeCSV,
		ResponseContentDisposition: fmt.Sprintf(constant.ContentDispositionPattern, params.FileName),
		ExpiresIn:                  params.ExpiresIn,
	})
	if err != nil {
		err = errs.Wrap(errs.S3Internal, "generate presigned url", err)
		return "", err
	}

	return presignURL, nil
}
