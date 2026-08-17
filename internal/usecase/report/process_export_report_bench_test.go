package report

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/bytedance/sonic"

	"github.com/fikrimohammad/efficient-report-exporter/internal/config"
	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
	"github.com/fikrimohammad/efficient-report-exporter/internal/mock"
	"github.com/fikrimohammad/efficient-report-exporter/internal/model"
	"github.com/fikrimohammad/efficient-report-exporter/internal/repository"
	"github.com/fikrimohammad/go-dev-sdk/confloader"
	"go.uber.org/mock/gomock"
)

// ---------------------------------------------------------------------------
// Benchmark harness
//
// These benchmarks measure the in-process export pipeline (streaming, line
// flattening, CSV/zip formatting, backpressure) in isolation from the network.
// MySQL rows are generated lazily page-by-page (mimicking keyset pagination) so
// the harness holds O(1) state, and the S3 upload drains into io.Discard so no
// network I/O is measured. This isolates the compute/memory characteristics the
// article claims: throughput and flat memory.
// ---------------------------------------------------------------------------

// benchReportSource lazily generates n deterministic reports spread uniformly
// across [start, end). Rows are produced on demand by QueryReport, never held in
// memory as a whole, matching the production keyset-pagination read path.
type benchReportSource struct {
	shopID      int64
	n           int
	start       time.Time
	end         time.Time
	step        time.Duration
	detailsJSON []byte
}

// indexFor returns the first row index whose settlement time is >= t.
func (s *benchReportSource) indexFor(t time.Time) int {
	if !t.After(s.start) {
		return 0
	}
	if t.After(s.end) {
		return s.n
	}
	d := t.Sub(s.start)
	return int((d + s.step - 1) / s.step)
}

func (s *benchReportSource) makeReport(i int) *model.Report {
	ts := s.start.Add(time.Duration(i) * s.step).UnixMilli()
	return &model.Report{
		ID:                  int64(i + 1),
		ShopID:              s.shopID,
		OrderID:             int64(i + 1),
		OrderCreationTime:   ts,
		OrderPaymentTime:    ts,
		OrderSettlementTime: ts,
		FeeID:               int64(i + 1),
		Details:             bytes.Clone(s.detailsJSON),
	}
}

// query returns a keyset page: rows with id > lastID and settlement time within
// [start, end), up to limit rows.
func (s *benchReportSource) query(start, end time.Time, lastID int64, limit int) []*model.Report {
	if limit <= 0 {
		limit = repository.MaxQueryReportLimit
	}
	iStart := s.indexFor(start)
	iEnd := s.indexFor(end)
	from := iStart
	if int64(from) < lastID {
		from = int(lastID)
	}
	if from >= iEnd {
		return nil
	}
	if limit > iEnd-from {
		limit = iEnd - from
	}
	out := make([]*model.Report, 0, limit)
	for i := from; i < from+limit; i++ {
		out = append(out, s.makeReport(i))
	}
	return out
}

// benchMySQL implements repository.MySQL with a lock-free lazy row source, so
// the batched path's concurrent batch fetches are measured fairly (no mutex
// serialization).
type benchMySQL struct {
	src *benchReportSource
}

func (m *benchMySQL) QueryReport(_ context.Context, f repository.QueryReportFilter) ([]*model.Report, error) {
	return m.src.query(*f.OrderSettlementTimeRange.StartTime, *f.OrderSettlementTimeRange.EndTime, f.LastReportID, f.Limit), nil
}

func (m *benchMySQL) CountReport(_ context.Context, f repository.QueryReportFilter) (int64, error) {
	return int64(m.src.indexFor(*f.OrderSettlementTimeRange.EndTime) - m.src.indexFor(*f.OrderSettlementTimeRange.StartTime)), nil
}

func (m *benchMySQL) QueryExportReportJob(context.Context, repository.QueryExportReportJobFilter) ([]*model.ExportReportJob, error) {
	return nil, nil
}

func (m *benchMySQL) InsertExportReportJob(context.Context, repository.InsertExportReportJobParams) (*model.ExportReportJob, error) {
	return &model.ExportReportJob{ID: 1}, nil
}

func (m *benchMySQL) UpdateExportReportJob(context.Context, repository.UpdateExportReportJobParams) error {
	return nil
}

// benchS3 implements repository.S3 by discarding uploaded bytes, so the upload
// stage drains the io.Pipe without network cost.
type benchS3 struct{}

func (benchS3) UploadReportFile(_ context.Context, p repository.UploadReportFileParams) error {
	_, err := io.Copy(io.Discard, p.FileData)
	return err
}

func (benchS3) GeneratePresignedDownloadURL(context.Context, repository.GeneratePresignedDownloadURLParams) (string, error) {
	return "", nil
}

var _ repository.S3 = benchS3{}

func newBenchDynamicLoader(b *testing.B, maxSingleFileRows, maxBatchWorkers int) *confloader.Loader[config.DynamicConfig] {
	mc := mock.NewConfigClient(map[string]string{
		"process_export_report/query_limit_per_page":       "2000",
		"process_export_report/max_time_range_per_batch":   "2h0m0s",
		"process_export_report/max_batch_pipeline_workers": fmt.Sprintf("%d", maxBatchWorkers),
		"process_export_report/max_single_file_rows":       fmt.Sprintf("%d", maxSingleFileRows),
		"process_export_report/request_lock_ttl":           "5s",
		"process_export_report/process_lock_ttl":           "1m0s",
		"process_export_report/csv_write_buf_size":         "1048576",
	})
	cfg := confloader.Config{
		Provider:         confloader.ProviderEtcd,
		Endpoint:         "localhost:2379",
		AuthClientID:     "test",
		AuthClientSecret: "test",
		Namespace:        "testns",
		Watcher:          confloader.DefaultWatcherConfig(),
	}
	ldr, err := confloader.New[config.DynamicConfig](
		context.Background(), cfg,
		confloader.WithClient(mc),
		confloader.WithInitialFetch(false),
	)
	if err != nil {
		b.Fatalf("newBenchDynamicLoader: %v", err)
	}
	return ldr
}

func newBenchExporter(b *testing.B, n, maxSingleFileRows int) (*reportExporter, time.Time, time.Time) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	detailsJSON, err := sonic.Marshal(model.ReportFeeDetails{
		{OrderDetailID: 1, CategoryID: 10, ProductID: 100, ProductPriceAmount: 9.99, PromoAmount: 1, FeeBaseAmount: 8.99, FeeFinalAmount: 0.5},
		{OrderDetailID: 2, CategoryID: 20, ProductID: 200, ProductPriceAmount: 19.99, PromoAmount: 0, FeeBaseAmount: 19.99, FeeFinalAmount: 1.5},
	})
	if err != nil {
		b.Fatalf("marshal bench details: %v", err)
	}

	src := &benchReportSource{
		shopID:      100,
		n:           n,
		start:       start,
		end:         end,
		step:        end.Sub(start) / time.Duration(n),
		detailsJSON: detailsJSON,
	}

	mq := mock.NewMockMQ(gomock.NewController(b))
	mq.EXPECT().PublishExportReportDoneMsg(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	re := &reportExporter{
		mysqlRepository: &benchMySQL{src: src},
		mqRepository:    mq,
		s3Repository:    benchS3{},
		dynamicConfig:   newBenchDynamicLoader(b, maxSingleFileRows, constant.DefaultMaxBatchPipelineWorkers),
	}

	return re, start, end
}

func runBench(b *testing.B, n, maxSingleFileRows int) {
	re, start, end := newBenchExporter(b, n, maxSingleFileRows)
	defer func() { _ = re.dynamicConfig.Stop() }()

	b.ReportAllocs()
	b.ReportMetric(float64(n), "rows/op")
	b.ResetTimer()

	cpuStart := readCPUSeconds()
	for i := 0; i < b.N; i++ {
		if err := re.runExportReportPipeline(context.Background(), 100, start, end, 1); err != nil {
			b.Fatalf("runExportReportPipeline: %v", err)
		}
	}
	cpuEnd := readCPUSeconds()

	reportCPU(b, cpuStart, cpuEnd)
	b.ReportMetric(float64(n)*float64(b.N)/b.Elapsed().Seconds(), "rows_per_sec")
}

// BenchmarkPipelineSingle measures the single-CSV path (streaming, no zip).
func BenchmarkPipelineSingle(b *testing.B) {
	for _, n := range []int{10_000, 50_000, 100_000} {
		b.Run(fmt.Sprintf("rows=%d", n), func(b *testing.B) {
			runBench(b, n, constant.DefaultMaxSingleFileRows)
		})
	}
}

// BenchmarkPipelineBatched measures the date-batched path (fan-out + zip).
// maxSingleFileRows=1 forces the batched path regardless of row count.
func BenchmarkPipelineBatched(b *testing.B) {
	for _, n := range []int{100_000, 500_000, 1_000_000} {
		b.Run(fmt.Sprintf("rows=%d", n), func(b *testing.B) {
			runBench(b, n, 1)
		})
	}
}

// peakMem captures the distinct notions of memory a process can use. Peak live
// heap (HeapAlloc) is only the reachable-object waterline; the OS charges the
// process for much more (RSS / Sys), because Go retains freed heap as idle
// spans instead of returning it eagerly to the OS.
type peakMem struct {
	heapAlloc uint64 // peak reachable objects (live set)
	heapSys   uint64 // peak heap memory obtained from the OS (in-use + idle)
	sys       uint64 // peak total memory obtained from the OS (all runtime areas)
	rss       uint64 // peak resident set size as reported by the OS
}

func readRSS() uint64 {
	data, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) < 2 {
		return 0
	}
	residentPages, _ := strconv.ParseUint(fields[1], 10, 64)
	return residentPages * uint64(os.Getpagesize())
}

// readCPUSeconds returns the process's total CPU time (user + system) in
// seconds via getrusage(RUSAGE_SELF).
func readCPUSeconds() float64 {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	return float64(ru.Utime.Sec) + float64(ru.Utime.Usec)/1e6 +
		float64(ru.Stime.Sec) + float64(ru.Stime.Usec)/1e6
}

// reportCPU reports CPU time per op and average CPU utilization (percentage of
// one core) for the region between cpuStart and cpuEnd (in seconds).
func reportCPU(b *testing.B, cpuStart, cpuEnd float64) {
	cpuSeconds := cpuEnd - cpuStart
	wallSeconds := b.Elapsed().Seconds()
	b.ReportMetric(cpuSeconds*1e9/float64(b.N), "cpu_ns/op")
	if wallSeconds > 0 {
		b.ReportMetric(cpuSeconds/wallSeconds*100, "cpu_pct")
	}
}

// runPipelinePeakMem runs the pipeline once while sampling memory every 1ms,
// capturing the peak of each memory metric during the run.
func runPipelinePeakMem(re *reportExporter, start, end time.Time) peakMem {
	runtime.GC()
	debug.FreeOSMemory()

	var (
		stop = make(chan struct{})
		wg   sync.WaitGroup
		mu   sync.Mutex
		p    peakMem
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		t := time.NewTicker(time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				var ms runtime.MemStats
				runtime.ReadMemStats(&ms)
				mu.Lock()
				if ms.HeapAlloc > p.heapAlloc {
					p.heapAlloc = ms.HeapAlloc
				}
				if ms.HeapSys > p.heapSys {
					p.heapSys = ms.HeapSys
				}
				if ms.Sys > p.sys {
					p.sys = ms.Sys
				}
				if rss := readRSS(); rss > p.rss {
					p.rss = rss
				}
				mu.Unlock()
			}
		}
	}()

	_ = re.runExportReportPipeline(context.Background(), 100, start, end, 1)

	close(stop)
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	return p
}

// BenchmarkPipelinePeakMem reports the four memory notions — live heap, heap
// from OS, total from OS, and RSS — so the "flat memory" claim is backed by the
// number the OS actually charges, not just the reachable-object waterline.
func BenchmarkPipelinePeakMem(b *testing.B) {
	for _, n := range []int{100_000, 1_000_000, 5_000_000} {
		b.Run(fmt.Sprintf("rows=%d", n), func(b *testing.B) {
			// Force the single-streaming path across all sizes for a clean line.
			re, start, end := newBenchExporter(b, n, 1_000_000_000)
			defer func() { _ = re.dynamicConfig.Stop() }()

			b.ReportMetric(float64(n), "rows/op")
			cpuStart := readCPUSeconds()
			for i := 0; i < b.N; i++ {
				p := runPipelinePeakMem(re, start, end)
				b.ReportMetric(float64(p.heapAlloc), "peak_heapalloc_B/op")
				b.ReportMetric(float64(p.heapSys), "peak_heapsys_B/op")
				b.ReportMetric(float64(p.sys), "peak_sys_B/op")
				b.ReportMetric(float64(p.rss), "peak_rss_B/op")
			}
			cpuEnd := readCPUSeconds()

			reportCPU(b, cpuStart, cpuEnd)
			b.ReportMetric(float64(n)*float64(b.N)/b.Elapsed().Seconds(), "rows_per_sec")
		})
	}
}

// BenchmarkPipelineBatchedPeakMem reports the peak live heap and RSS for the
// batched path, where each batch worker buffers its compressed CSV in memory
// before the zip stage assembles it. This captures the memory cost of the
// parallel-deflate change (a bounded per-batch buffer, not O(rows)).
func BenchmarkPipelineBatchedPeakMem(b *testing.B) {
	for _, n := range []int{500_000, 1_000_000} {
		b.Run(fmt.Sprintf("rows=%d", n), func(b *testing.B) {
			// maxSingleFileRows=1 forces the batched path at every size.
			re, start, end := newBenchExporter(b, n, 1)
			defer func() { _ = re.dynamicConfig.Stop() }()

			b.ReportMetric(float64(n), "rows/op")
			for i := 0; i < b.N; i++ {
				p := runPipelinePeakMem(re, start, end)
				b.ReportMetric(float64(p.heapAlloc), "peak_heapalloc_B/op")
				b.ReportMetric(float64(p.rss), "peak_rss_B/op")
			}
		})
	}
}
