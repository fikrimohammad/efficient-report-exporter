package s3

import (
	"context"

	"github.com/fikrimohammad/efficient-report-exporter/apperrors"
	"github.com/fikrimohammad/efficient-report-exporter/constant"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
	"github.com/fikrimohammad/go-dev-sdk/errs"
	commons3 "github.com/fikrimohammad/go-dev-sdk/s3"
)

func (r *repo) UploadReportFile(ctx context.Context, params repository.UploadReportFileParams) error {
	if params.FileData == nil {
		return errs.New(apperrors.InvalidArgument, "file_data is required")
	}

	if params.FileName == "" {
		return errs.New(apperrors.InvalidArgument, "file_name is required")
	}

	err := r.s3.UploadObject(ctx, commons3.UploadObjectParams{
		Bucket:      constant.ReportFileBucket,
		Key:         params.FileName,
		Body:        params.FileData,
		ContentType: constant.ContentTypeOctetStream,
	})
	if err != nil {
		err = errs.Wrap(apperrors.S3Internal, "upload report file", err)
		return err
	}

	return nil
}
