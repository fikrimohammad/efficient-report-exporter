package redis

import (
	"context"

	"github.com/fikrimohammad/efficient-report-exporter/repository"
	"github.com/fikrimohammad/go-dev-sdk/errs"
)

func (r *repo) LockExportReportProcess(ctx context.Context, params repository.LockExportReportProcess) error {
	key := exportReportProcessKey(params.JobID)
	ok, err := r.redisCli.SetNX(ctx, key, true, params.TTL).Result()
	if err != nil {
		err = errs.Wrap(errs.CacheInternal, "set process lock key", err)
		return err
	}

	if !ok {
		return errs.New(errs.InvalidArgument, "lock export report process already running")
	}

	return nil
}
