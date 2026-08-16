package redis

import (
	"context"
	"errors"

	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
	"github.com/fikrimohammad/efficient-report-exporter/internal/repository"
	"github.com/fikrimohammad/go-dev-sdk/errs/v2"
	"github.com/redis/go-redis/v9"
)

func (r *repo) UnlockExportReportProcess(ctx context.Context, params repository.UnlockExportReportProcess) error {
	key := exportReportProcessKey(params.JobID)
	val, err := r.redisCli.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return nil
	}
	if err != nil {
		return errs.Wrap(constant.CacheInternal, "get process lock key", err)
	}

	// Release the lock only if it is still held by this owner; the key may have
	// expired and been re-acquired by another consumer.
	if val != params.Token {
		return nil
	}

	if err := r.redisCli.Del(ctx, key).Err(); err != nil {
		return errs.Wrap(constant.CacheInternal, "delete process lock key", err)
	}

	return nil
}
