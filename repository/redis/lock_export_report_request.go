package redis

import (
	"context"

	"github.com/fikrimohammad/go-dev-sdk/errs"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
)

func (r *repo) LockExportReportRequest(ctx context.Context, params repository.LockExportReportRequest) error {
	key := exportReportRequestKey(params.RequestID)
	ok, err := r.redisCli.SetNX(ctx, key, true, params.TTL).Result()
	if err != nil {
		err = errs.Wrap(errs.CacheInternal, "set lock key", err)
		return err
	}

	if !ok {
		return errs.New(errs.InvalidArgument, "lock export request is already locked")
	}

	return nil
}
