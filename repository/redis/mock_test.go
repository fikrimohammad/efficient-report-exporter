package redis

import (
	"testing"

	"github.com/fikrimohammad/efficient-report-exporter/internal/mocks"
	"go.uber.org/mock/gomock"
)

func newMockRedisClient(t *testing.T) *mocks.MockRedisClient {
	t.Helper()
	return mocks.NewMockRedisClient(gomock.NewController(t))
}
