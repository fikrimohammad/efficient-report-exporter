//go:build integration

package integration

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	_ "github.com/go-sql-driver/mysql"

	"github.com/fikrimohammad/efficient-report-exporter/internal/config"
	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
	"github.com/fikrimohammad/efficient-report-exporter/internal/mock"
	"github.com/fikrimohammad/efficient-report-exporter/internal/repository"
	mysqlrepository "github.com/fikrimohammad/efficient-report-exporter/internal/repository/mysql"
	s3repository "github.com/fikrimohammad/efficient-report-exporter/internal/repository/s3"
	"github.com/fikrimohammad/go-dev-sdk/confloader"
	"github.com/fikrimohammad/go-dev-sdk/db"
	commonredis "github.com/fikrimohammad/go-dev-sdk/redis"
	rocketmqconsumer "github.com/fikrimohammad/go-dev-sdk/rocketmq/consumer"
	commons3 "github.com/fikrimohammad/go-dev-sdk/s3"
	"go.uber.org/mock/gomock"
)

const (
	// benchWindow is the settlement-time window rows are seeded across. At the
	// default 2h batch size this yields 12 batches in the batched path.
	benchWindow = 24 * time.Hour

	// benchDetailsPerRow is the fixed number of fee details per seeded row,
	// matching the shape of real data (1–5 details, mean ~3).
	benchDetailsPerRow = 3
)

var benchStart = time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

// benchShopIDFor maps a row count to a dedicated shop id, so each size gets its
// own seeded dataset and re-runs skip re-seeding.
func benchShopIDFor(n int) int64 {
	return int64(424200 + n/50_000)
}

// deps bundles the real connections a test needs.
type deps struct {
	infra     testInfra
	dbClient  db.DB
	s3Client  commons3.Client
	rawDB     *sql.DB
	mysqlRepo repository.MySQL
	s3Repo    repository.S3
}

// testInfra holds the connection settings for the test infrastructure, resolved
// either from environment variables (CI) or the app's config loader (local).
type testInfra struct {
	db      db.Config
	s3      commons3.Config
	redis   commonredis.Config
	mq      rocketmqconsumer.Config
	namesrv []string
}

// loadTestInfra resolves the infrastructure settings. When TEST_DB_HOST is set
// (CI, backed by docker-compose), settings come from environment variables so
// the tests don't need Infisical/etcd; otherwise the app's config loader is
// used, as in local development.
func loadTestInfra(t testing.TB) testInfra {
	t.Helper()
	if os.Getenv("TEST_DB_HOST") != "" {
		return envTestInfra()
	}
	return configTestInfra(t)
}

func configTestInfra(t testing.TB) testInfra {
	t.Helper()

	root := repoRoot()
	if root == "" {
		t.Fatal("repo root (go.mod) not found; run from within the repository")
	}
	loadDotEnv(root)
	if os.Getenv("CONFIG_PATH") == "" {
		_ = os.Setenv("CONFIG_PATH", filepath.Join(root, "config", "config.development.yaml"))
	}

	cfg, err := config.Load(context.Background())
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	t.Cleanup(func() { _ = cfg.Dynamic.Stop() })

	namesrv := []string{"127.0.0.1:9876"}
	if len(cfg.MQConsumers) > 0 {
		namesrv = cfg.MQConsumers[0].Endpoints
	}

	return testInfra{
		db:      cfg.DB,
		s3:      cfg.S3,
		redis:   cfg.Redis,
		mq:      cfg.MQConsumers[0],
		namesrv: namesrv,
	}
}

func envTestInfra() testInfra {
	namesrv := []string{getenv("TEST_MQ_NAMESRV", "127.0.0.1:9876")}

	return testInfra{
		db: db.Config{
			Driver:   "mysql",
			Host:     getenv("TEST_DB_HOST", "127.0.0.1"),
			Port:     atoi(getenv("TEST_DB_PORT", "3306")),
			Database: getenv("TEST_DB_NAME", "efficient_report_exporter"),
			Username: getenv("TEST_DB_USER", "root"),
			Password: getenv("TEST_DB_PASSWORD", "root"),
		},
		s3: commons3.Config{
			Region:          getenv("TEST_S3_REGION", "us-east-1"),
			Endpoint:        getenv("TEST_S3_ENDPOINT", "http://127.0.0.1:9000"),
			AccessKeyID:     getenv("TEST_S3_ACCESS_KEY", "minioadmin"),
			SecretAccessKey: getenv("TEST_S3_SECRET_KEY", "minioadmin"),
		},
		redis: commonredis.Config{
			Host: getenv("TEST_REDIS_HOST", "127.0.0.1"),
			Port: atoi(getenv("TEST_REDIS_PORT", "6379")),
		},
		mq: rocketmqconsumer.Config{
			Endpoints:        namesrv,
			Topic:            string(constant.MQTopicReporting),
			Group:            getenv("TEST_MQ_GROUP", "e2e_export_report_consumer"),
			Tags:             []string{string(constant.MQMsgTagExportReportProcess)},
			ConsumeFromWhere: rocketmqconsumer.ConsumeFromLast,
		},
		namesrv: namesrv,
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func atoi(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}

// setupDeps connects to MySQL and S3, provisions the report bucket, and returns
// the wired repositories plus a raw *sql.DB for seeding.
func setupDeps(t testing.TB) *deps {
	t.Helper()
	ctx := context.Background()

	infra := loadTestInfra(t)

	dbClient, err := db.Connect(infra.db)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	t.Cleanup(func() { _ = dbClient.Close() })

	s3Client, err := commons3.New(infra.s3)
	if err != nil {
		t.Fatalf("init s3: %v", err)
	}
	if err := ensureBucket(ctx, infra.s3); err != nil {
		t.Fatalf("ensure bucket: %v", err)
	}

	mysqlRepo, err := mysqlrepository.New(dbClient)
	if err != nil {
		t.Fatalf("mysql repo: %v", err)
	}
	s3Repo, err := s3repository.New(s3Client)
	if err != nil {
		t.Fatalf("s3 repo: %v", err)
	}

	rawDB, err := sql.Open("mysql", dbDSN(infra.db))
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	t.Cleanup(func() { _ = rawDB.Close() })
	if err := rawDB.PingContext(ctx); err != nil {
		t.Fatalf("ping raw db: %v", err)
	}

	return &deps{
		infra:     infra,
		dbClient:  dbClient,
		s3Client:  s3Client,
		rawDB:     rawDB,
		mysqlRepo: mysqlRepo,
		s3Repo:    s3Repo,
	}
}

// newMockMQ returns a repository.MQ mock that accepts any publish call.
func newMockMQ(t testing.TB) *mock.MockMQ {
	t.Helper()
	mq := mock.NewMockMQ(gomock.NewController(t))
	mq.EXPECT().PublishExportReportDoneMsg(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mq.EXPECT().PublishExportReportProcessMsg(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	return mq
}

// newMockRedis returns a repository.Redis mock whose lock/unlock always succeed.
func newMockRedis(t testing.TB) *mock.MockRedis {
	t.Helper()
	r := mock.NewMockRedis(gomock.NewController(t))
	r.EXPECT().LockExportReportProcess(gomock.Any(), gomock.Any()).Return("token", nil).AnyTimes()
	r.EXPECT().UnlockExportReportProcess(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	r.EXPECT().LockExportReportRequest(gomock.Any(), gomock.Any()).Return("token", nil).AnyTimes()
	r.EXPECT().UnlockExportReportRequest(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	return r
}

func newDynamicLoader(t testing.TB, maxSingleFileRows, maxBatchWorkers int) *confloader.Loader[config.DynamicConfig] {
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
		t.Fatalf("newDynamicLoader: %v", err)
	}
	return ldr
}

func repoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// loadDotEnv exports KEY=VALUE lines from <root>/.env only when the key is not
// already set, so the Infisical machine-identity env vars are available to the
// config loader during tests (the Makefile does the same for run targets).
func loadDotEnv(root string) {
	data, err := os.ReadFile(filepath.Join(root, ".env"))
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		k, v := strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1])
		if k != "" && os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
}

func dbDSN(cfg db.Config) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4",
		cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
}

// ensureBucket provisions the report bucket if it doesn't exist. The app's S3
// credentials are scoped to object operations (no s3:CreateBucket/ListBucket),
// so provisioning falls back to admin credentials when available — mirroring how
// buckets are created out-of-band in a real deployment. Set MINIO_ROOT_USER /
// MINIO_ROOT_PASSWORD to allow creating a missing bucket.
func ensureBucket(ctx context.Context, cfg commons3.Config) error {
	if err := headOrCreateBucket(ctx, cfg, cfg.AccessKeyID, cfg.SecretAccessKey); err == nil {
		return nil
	}
	adminUser := os.Getenv("MINIO_ROOT_USER")
	adminPass := os.Getenv("MINIO_ROOT_PASSWORD")
	if adminUser == "" || adminPass == "" {
		return fmt.Errorf("bucket %q is missing and MINIO_ROOT_USER/MINIO_ROOT_PASSWORD are not set to provision it", constant.ReportFileBucket)
	}
	return headOrCreateBucket(ctx, cfg, adminUser, adminPass)
}

func headOrCreateBucket(ctx context.Context, cfg commons3.Config, accessKey, secretKey string) error {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return err
	}
	client := awss3.NewFromConfig(awsCfg, func(o *awss3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = true
	})
	if _, err := client.HeadBucket(ctx, &awss3.HeadBucketInput{Bucket: aws.String(constant.ReportFileBucket)}); err == nil {
		return nil
	}
	_, err = client.CreateBucket(ctx, &awss3.CreateBucketInput{Bucket: aws.String(constant.ReportFileBucket)})
	return err
}

// seedBenchRows idempotently seeds n report rows for shopID across [start, end).
// If the shop already holds exactly n rows it is left untouched, so repeated
// benchmark runs are cheap.
func seedBenchRows(ctx context.Context, rawDB *sql.DB, shopID int64, n int, start, end time.Time) error {
	var cnt int64
	if err := rawDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM report WHERE shop_id = ?", shopID).Scan(&cnt); err != nil {
		return err
	}
	if cnt == int64(n) {
		return nil
	}

	if _, err := rawDB.ExecContext(ctx, "DELETE FROM report WHERE shop_id = ?", shopID); err != nil {
		return err
	}

	const batchSize = 1000
	step := end.Sub(start) / time.Duration(n)
	placeholders := make([]string, 0, batchSize)
	args := make([]any, 0, batchSize*7)

	flush := func() error {
		if len(placeholders) == 0 {
			return nil
		}
		query := "INSERT INTO report (shop_id, order_id, order_creation_time, order_payment_time, order_settlement_time, fee_id, details, creation_time, update_time) VALUES " +
			strings.Join(placeholders, ",")
		if _, err := rawDB.ExecContext(ctx, query, args...); err != nil {
			return err
		}
		placeholders = placeholders[:0]
		args = args[:0]
		return nil
	}

	for i := 0; i < n; i++ {
		settle := start.Add(time.Duration(i) * step)
		pay := settle.Add(-time.Hour)
		create := pay.Add(-30 * time.Minute)
		details := fmt.Sprintf(
			`[{"order_detail_id":%d,"category_id":1,"product_id":1,"product_price_amount":9.99,"promo_amount":1,"fee_base_amount":8.99,"fee_final_amount":0.5},{"order_detail_id":%d,"category_id":2,"product_id":2,"product_price_amount":19.99,"promo_amount":0,"fee_base_amount":19.99,"fee_final_amount":1.5},{"order_detail_id":%d,"category_id":3,"product_id":3,"product_price_amount":29.99,"promo_amount":0,"fee_base_amount":29.99,"fee_final_amount":2}]`,
			i*benchDetailsPerRow+1, i*benchDetailsPerRow+2, i*benchDetailsPerRow+3)

		placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args, shopID, int64(i+1), create.UnixMilli(), pay.UnixMilli(), settle.UnixMilli(), int64(i+1), details, settle.UnixMilli(), settle.UnixMilli())

		if len(placeholders) >= batchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	return flush()
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
