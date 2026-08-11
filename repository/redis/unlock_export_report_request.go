package redis

import (
	"context"
	"errors"

	"github.com/fikrimohammad/efficient-report-exporter/repository"
	"github.com/fikrimohammad/go-dev-sdk/errs"
	"github.com/redis/go-redis/v9"
)

func (r *repo) UnlockExportReportRequest(ctx context.Context, params repository.UnlockExportReportRequest) error {
	key := exportReportRequestKey(params.RequestID)
	isLocked, err := r.redisCli.Get(ctx, key).Bool()
	if err != nil && !errors.Is(err, redis.Nil) {
		err = errs.Wrap(errs.CacheInternal, "get lock key", err)
		return err
	}

	if !isLocked {
		return nil
	}

	if err := r.redisCli.Del(ctx, key).Err(); err != nil {
		err = errs.Wrap(errs.CacheInternal, "delete lock key", err)
		return err
	}

	return nil
}
