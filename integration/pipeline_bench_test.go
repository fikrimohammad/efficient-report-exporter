//go:build integration

package integration

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
	"github.com/fikrimohammad/efficient-report-exporter/internal/model"
	"github.com/fikrimohammad/efficient-report-exporter/internal/repository"
	"github.com/fikrimohammad/efficient-report-exporter/internal/usecase"
	reportusecase "github.com/fikrimohammad/efficient-report-exporter/internal/usecase/report"
)

// realBenchEnv bundles the exported report use case (backed by real MySQL and
// S3) with a pre-inserted job and the counters used to attribute wall-clock
// time to the DB fetch and the S3 upload.
type realBenchEnv struct {
	uc          usecase.Report
	rawDB       *sql.DB
	jobID       int64
	fetchNanos  *atomic.Int64
	uploadNanos *atomic.Int64
}

func setupRealBench(b *testing.B, n, maxSingleFileRows int) *realBenchEnv {
	b.Helper()
	ctx := context.Background()

	d := setupDeps(b)

	shopID := benchShopIDFor(n)
	start := benchStart
	end := benchStart.Add(benchWindow)
	if err := seedBenchRows(ctx, d.rawDB, shopID, n, start, end); err != nil {
		b.Fatalf("seed %d rows for shop %d: %v", n, shopID, err)
	}

	// Wrap the real repositories to attribute wall-clock time to the DB fetch
	// and the S3 upload, so the end-to-end number can be decomposed.
	var fetchNanos, uploadNanos atomic.Int64
	mysqlRepo := &timedMySQL{MySQL: d.mysqlRepo, fetch: &fetchNanos, count: &fetchNanos}
	s3Repo := &timedS3{S3: d.s3Repo, upload: &uploadNanos}

	dl := newDynamicLoader(b, maxSingleFileRows, constant.DefaultMaxBatchPipelineWorkers)
	b.Cleanup(func() { _ = dl.Stop() })

	uc, err := reportusecase.New(mysqlRepo, newMockMQ(b), newMockRedis(b), s3Repo, dl)
	if err != nil {
		b.Fatalf("report use case: %v", err)
	}

	const jobID = int64(1)
	insertJob(b, d.rawDB, jobID, shopID, start, end)
	b.Cleanup(func() { _ = deleteJob(d.rawDB, jobID) })

	return &realBenchEnv{
		uc:          uc,
		rawDB:       d.rawDB,
		jobID:       jobID,
		fetchNanos:  &fetchNanos,
		uploadNanos: &uploadNanos,
	}
}

func runRealBench(b *testing.B, n, maxSingleFileRows int) {
	env := setupRealBench(b, n, maxSingleFileRows)
	ctx := context.Background()

	b.ReportMetric(float64(n), "rows/op")
	b.ResetTimer()

	cpuStart := readCPUSeconds()
	for i := 0; i < b.N; i++ {
		if err := resetJob(ctx, env.rawDB, env.jobID); err != nil {
			b.Fatalf("reset job: %v", err)
		}
		if err := env.uc.ProcessExportReport(ctx, usecase.ProcessExportReportParams{JobID: env.jobID}); err != nil {
			b.Fatalf("ProcessExportReport: %v", err)
		}
	}
	cpuEnd := readCPUSeconds()

	reportCPU(b, cpuStart, cpuEnd)
	b.ReportMetric(float64(n)*float64(b.N)/b.Elapsed().Seconds(), "rows_per_sec")
	b.ReportMetric(float64(env.fetchNanos.Load()), "db_fetch_ns/op")
	b.ReportMetric(float64(env.uploadNanos.Load()), "s3_upload_ns/op")
}

// insertJob inserts a processing export job for the given shop/range, matching
// what RequestExportReport persists, so ProcessExportReport has a job to run.
func insertJob(t testing.TB, rawDB *sql.DB, jobID, shopID int64, start, end time.Time) {
	t.Helper()
	_, err := rawDB.ExecContext(context.Background(),
		`INSERT INTO export_report_job (id, request_id, shop_id, start_time, end_time, status, extra, creation_time)
		 VALUES (?, ?, ?, ?, ?, 'processing', '{}', ?)`,
		jobID, jobID, shopID, start.UnixMilli(), end.UnixMilli(), time.Now().UnixMilli())
	if err != nil {
		t.Fatalf("insert job: %v", err)
	}
}

// resetJob returns a job to the processing state so a benchmark iteration can
// run the pipeline again.
func resetJob(ctx context.Context, rawDB *sql.DB, jobID int64) error {
	_, err := rawDB.ExecContext(ctx,
		`UPDATE export_report_job SET status = 'processing', extra = '{}', update_time = NULL WHERE id = ?`, jobID)
	return err
}

func deleteJob(rawDB *sql.DB, jobID int64) error {
	_, err := rawDB.ExecContext(context.Background(), `DELETE FROM export_report_job WHERE id = ?`, jobID)
	return err
}

// timedMySQL wraps a repository.MySQL and accumulates the wall-clock time spent
// in CountReport + QueryReport (the DB fetch). QueryReport is called from
// concurrent batch goroutines, so the counters are atomic.
type timedMySQL struct {
	repository.MySQL
	fetch *atomic.Int64
	count *atomic.Int64
}

func (m *timedMySQL) QueryReport(ctx context.Context, f repository.QueryReportFilter) ([]*model.Report, error) {
	start := time.Now()
	rows, err := m.MySQL.QueryReport(ctx, f)
	m.fetch.Add(time.Since(start).Nanoseconds())
	return rows, err
}

func (m *timedMySQL) CountReport(ctx context.Context, f repository.QueryReportFilter) (int64, error) {
	start := time.Now()
	c, err := m.MySQL.CountReport(ctx, f)
	m.count.Add(time.Since(start).Nanoseconds())
	return c, err
}

// timedS3 wraps a repository.S3 and accumulates wall-clock time spent uploading.
type timedS3 struct {
	repository.S3
	upload *atomic.Int64
}

func (s *timedS3) UploadReportFile(ctx context.Context, p repository.UploadReportFileParams) error {
	start := time.Now()
	err := s.S3.UploadReportFile(ctx, p)
	s.upload.Add(time.Since(start).Nanoseconds())
	return err
}

// BenchmarkRealSingle measures the single-CSV path against real MySQL + MinIO.
// maxSingleFileRows is forced huge so the single path is used at every size.
func BenchmarkRealSingle(b *testing.B) {
	for _, n := range []int{100_000, 500_000, 1_000_000} {
		b.Run(fmt.Sprintf("rows=%d", n), func(b *testing.B) {
			runRealBench(b, n, 1_000_000_000)
		})
	}
}

// BenchmarkRealBatched measures the date-batched + zip path against real
// MySQL + MinIO. maxSingleFileRows=1 forces the batched path at every size.
func BenchmarkRealBatched(b *testing.B) {
	for _, n := range []int{100_000, 500_000, 1_000_000} {
		b.Run(fmt.Sprintf("rows=%d", n), func(b *testing.B) {
			runRealBench(b, n, 1)
		})
	}
}
