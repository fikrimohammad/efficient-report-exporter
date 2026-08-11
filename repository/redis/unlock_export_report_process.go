package redis

import (
	"context"
	"errors"

	"github.com/fikrimohammad/efficient-report-exporter/common/errs"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
	"github.com/redis/go-redis/v9"
)

func (r *repo) UnlockExportReportProcess(ctx context.Context, params repository.UnlockExportReportProcess) error {
	key := exportReportProcessKey(params.JobID)
	isLocked, err := r.redisCli.Get(ctx, key).Bool()
	if err != nil && !errors.Is(err, redis.Nil) {
		err = errs.Wrap(errs.CacheInternal, "get process lock key", err)
		return err
	}

	if !isLocked {
		return nil
	}

	if err := r.redisCli.Del(ctx, key).Err(); err != nil {
		err = errs.Wrap(errs.CacheInternal, "delete process lock key", err)
		return err
	}

	return nil
}
