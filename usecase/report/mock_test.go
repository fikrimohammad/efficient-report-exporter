package report

import (
	"context"
	"fmt"
	"sync"

	"github.com/fikrimohammad/go-dev-sdk/confloader"
	"github.com/fikrimohammad/go-dev-sdk/confloader/client"
	"github.com/fikrimohammad/efficient-report-exporter/config"
	"github.com/fikrimohammad/efficient-report-exporter/constant"
	"github.com/fikrimohammad/efficient-report-exporter/model"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
)

// ---------------------------------------------------------------------------
// mockMySQL
// ---------------------------------------------------------------------------

type mockMySQL struct {
	mu                    sync.Mutex
	queryReportFn         func(ctx context.Context, filter repository.QueryReportFilter) ([]*model.Report, error)
	queryExportReportJob  func(ctx context.Context, filter repository.QueryExportReportJobFilter) ([]*model.ExportReportJob, error)
	insertExportReportJob func(ctx context.Context, params repository.InsertExportReportJobParams) (*model.ExportReportJob, error)
	updateExportReportJob func(ctx context.Context, params repository.UpdateExportReportJobParams) error
}

func (m *mockMySQL) QueryReport(ctx context.Context, filter repository.QueryReportFilter) ([]*model.Report, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.queryReportFn(ctx, filter)
}

func (m *mockMySQL) QueryExportReportJob(ctx context.Context, filter repository.QueryExportReportJobFilter) ([]*model.ExportReportJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.queryExportReportJob(ctx, filter)
}

func (m *mockMySQL) InsertExportReportJob(ctx context.Context, params repository.InsertExportReportJobParams) (*model.ExportReportJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.insertExportReportJob(ctx, params)
}

func (m *mockMySQL) UpdateExportReportJob(ctx context.Context, params repository.UpdateExportReportJobParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.updateExportReportJob(ctx, params)
}

func defaultMockMySQL() *mockMySQL {
	return &mockMySQL{
		queryReportFn: func(_ context.Context, _ repository.QueryReportFilter) ([]*model.Report, error) {
			return nil, nil
		},
		queryExportReportJob: func(_ context.Context, _ repository.QueryExportReportJobFilter) ([]*model.ExportReportJob, error) {
			return nil, nil
		},
		insertExportReportJob: func(_ context.Context, _ repository.InsertExportReportJobParams) (*model.ExportReportJob, error) {
			return &model.ExportReportJob{ID: 1}, nil
		},
		updateExportReportJob: func(_ context.Context, _ repository.UpdateExportReportJobParams) error {
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// mockMQ
// ---------------------------------------------------------------------------

type mockMQ struct {
	mu                sync.Mutex
	publishProcessMsg func(ctx context.Context, msg model.ExportReportProcessMessage) error
	publishDoneMsg    func(ctx context.Context, msg model.ExportReportDoneMessage) error
}

func (m *mockMQ) PublishExportReportProcessMsg(ctx context.Context, msg model.ExportReportProcessMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.publishProcessMsg(ctx, msg)
}

func (m *mockMQ) PublishExportReportDoneMsg(ctx context.Context, msg model.ExportReportDoneMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.publishDoneMsg(ctx, msg)
}

func defaultMockMQ() *mockMQ {
	return &mockMQ{
		publishProcessMsg: func(_ context.Context, _ model.ExportReportProcessMessage) error {
			return nil
		},
		publishDoneMsg: func(_ context.Context, _ model.ExportReportDoneMessage) error {
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// mockRedis
// ---------------------------------------------------------------------------

type mockRedis struct {
	mu              sync.Mutex
	lockRequestFn   func(ctx context.Context, params repository.LockExportReportRequest) error
	unlockRequestFn func(ctx context.Context, params repository.UnlockExportReportRequest) error
	lockProcessFn   func(ctx context.Context, params repository.LockExportReportProcess) error
	unlockProcessFn func(ctx context.Context, params repository.UnlockExportReportProcess) error
}

func (m *mockRedis) LockExportReportRequest(ctx context.Context, params repository.LockExportReportRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lockRequestFn(ctx, params)
}

func (m *mockRedis) UnlockExportReportRequest(ctx context.Context, params repository.UnlockExportReportRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.unlockRequestFn(ctx, params)
}

func (m *mockRedis) LockExportReportProcess(ctx context.Context, params repository.LockExportReportProcess) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lockProcessFn(ctx, params)
}

func (m *mockRedis) UnlockExportReportProcess(ctx context.Context, params repository.UnlockExportReportProcess) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.unlockProcessFn(ctx, params)
}

func defaultMockRedis() *mockRedis {
	return &mockRedis{
		lockRequestFn:   func(_ context.Context, _ repository.LockExportReportRequest) error { return nil },
		unlockRequestFn: func(_ context.Context, _ repository.UnlockExportReportRequest) error { return nil },
		lockProcessFn:   func(_ context.Context, _ repository.LockExportReportProcess) error { return nil },
		unlockProcessFn: func(_ context.Context, _ repository.UnlockExportReportProcess) error { return nil },
	}
}

// ---------------------------------------------------------------------------
// mockS3
// ---------------------------------------------------------------------------

type mockS3 struct {
	mu                             sync.Mutex
	uploadReportFn                 func(ctx context.Context, params repository.UploadReportFileParams) error
	generatePresignedDownloadURLFn func(ctx context.Context, params repository.GeneratePresignedDownloadURLParams) (string, error)
}

func (m *mockS3) UploadReportFile(ctx context.Context, params repository.UploadReportFileParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.uploadReportFn(ctx, params)
}

func (m *mockS3) GeneratePresignedDownloadURL(ctx context.Context, params repository.GeneratePresignedDownloadURLParams) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.generatePresignedDownloadURLFn(ctx, params)
}

func defaultMockS3() *mockS3 {
	return &mockS3{
		uploadReportFn: func(_ context.Context, _ repository.UploadReportFileParams) error { return nil },
		generatePresignedDownloadURLFn: func(_ context.Context, _ repository.GeneratePresignedDownloadURLParams) (string, error) {
			return "https://s3.example.com/reports/test.csv?X-Amz-Signature=abc", nil
		},
	}
}

// ---------------------------------------------------------------------------
// mock confloader backend for DynamicConfig
// ---------------------------------------------------------------------------

type mockConfigClient struct {
	mu    sync.Mutex
	store map[string]string
}

func (m *mockConfigClient) Fetch(_ context.Context, folder, key string) (client.Fetched, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	path := folder + "/" + key
	v, ok := m.store[path]
	if !ok {
		return client.Fetched{}, fmt.Errorf("confloader/mock: key %s not found", path)
	}
	return client.Fetched{Value: v, Revision: "1"}, nil
}

func (m *mockConfigClient) Close() error { return nil }

// newTestDynamicLoader creates a real confloader.Loader[DynamicConfig]
// backed by an in-memory mock client. Panics on failure (test helper).
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
	mc := &mockConfigClient{store: map[string]string{
		"process_export_report/query_limit_per_page": fmt.Sprintf("%d", ps),
		"process_export_report/report_line_workers":  "4",
		"process_export_report/report_csv_workers":   "4",
		"process_export_report/request_lock_ttl":     "5s",
		"process_export_report/process_lock_ttl":     "1m0s",
		"process_export_report/csv_write_buf_size":   "1048576",
	}}
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

// Verify that mock types satisfy the repository interfaces.
var (
	_ repository.MySQL = (*mockMySQL)(nil)
	_ repository.MQ    = (*mockMQ)(nil)
	_ repository.Redis = (*mockRedis)(nil)
	_ repository.S3    = (*mockS3)(nil)
)

var (
	_ = constant.DefaultQueryLimitPerPage
	_ = constant.DefaultReportLineWorkers
	_ = constant.DefaultReportCSVWorkers
	_ = constant.DefaultRequestLockTTL
	_ = constant.DefaultProcessLockTTL
	_ = constant.DefaultCSVWriteBufSize
	_ = constant.SingleRowQueryLimit
)

func ptr(v int64) *int64 { return &v }
