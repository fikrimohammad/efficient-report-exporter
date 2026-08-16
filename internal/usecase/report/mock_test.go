package report

import (
	"context"
	"fmt"

	"github.com/fikrimohammad/efficient-report-exporter/internal/config"
	"github.com/fikrimohammad/efficient-report-exporter/internal/mock"
	"github.com/fikrimohammad/go-dev-sdk/confloader"
	"go.uber.org/mock/gomock"
)

// newTestDynamicLoader creates a real confloader.Loader[DynamicConfig]
// backed by an in-memory client. Panics on failure (test helper).
// Optional pageSize overrides default query_limit_per_page (default 100).
func newTestDynamicLoader(t interface{ Fatalf(string, ...interface{}) }, pageSize ...int) *confloader.Loader[config.DynamicConfig] {
	ps := 100
	if len(pageSize) > 0 && pageSize[0] > 0 {
		ps = pageSize[0]
	}

	cfg := confloader.Config{
		Provider:         confloader.ProviderEtcd,
		Endpoint:         "localhost:2379",
		AuthClientID:     "test",
		AuthClientSecret: "test",
		Namespace:        "testns",
		Watcher:          confloader.DefaultWatcherConfig(),
	}
	mc := mock.NewConfigClient(map[string]string{
		"process_export_report/query_limit_per_page":       fmt.Sprintf("%d", ps),
		"process_export_report/max_time_range_per_batch":   "2h0m0s",
		"process_export_report/max_batch_pipeline_workers": "8",
		"process_export_report/max_single_file_rows":       "100000",
		"process_export_report/request_lock_ttl":           "5s",
		"process_export_report/process_lock_ttl":           "1m0s",
		"process_export_report/csv_write_buf_size":         "1048576",
	})
	ldr, err := confloader.New[config.DynamicConfig](
		context.Background(), cfg,
		confloader.WithClient(mc),
		confloader.WithInitialFetch(false),
	)
	if err != nil {
		t.Fatalf("newTestDynamicLoader: %v", err)
	}
	return ldr
}

func ptr(v int64) *int64 { return &v }

// allowRedisLocks configures a MockRedis so that lock/unlock succeed for any
// job/request.
func allowRedisLocks(r *mock.MockRedis) {
	r.EXPECT().LockExportReportProcess(gomock.Any(), gomock.Any()).Return("token", nil).AnyTimes()
	r.EXPECT().UnlockExportReportProcess(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	r.EXPECT().LockExportReportRequest(gomock.Any(), gomock.Any()).Return("token", nil).AnyTimes()
	r.EXPECT().UnlockExportReportRequest(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
}
