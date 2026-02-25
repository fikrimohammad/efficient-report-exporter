# efficient-report-exporter

> Stream millions of financial report rows to CSV — without breaking a sweat.

A high-throughput HTTP service that exports large financial datasets as CSV files using a fully concurrent, memory-efficient streaming pipeline. No buffering. No OOM. Just data flowing from MySQL to the client.

---

## How it works

Instead of loading rows into memory, the export runs as a **three-stage concurrent pipeline**:

```
MySQL
  └─► [DB scan goroutine] ──────────────────► reportDataStream
                                                      │
                                              [32 worker goroutines]
                                              (explode JSON details)
                                                      │
                                                      ▼
                                             reportLineDataStream
                                                      │
                                              [CSV writer goroutine]
                                                      │
                                                      ▼
                                                  io.Pipe ──► HTTP (chunked)
```

Each stage is connected by a typed channel pipe. Workers fan out to flatten each report's `details` JSON array into individual rows. The CSV writer streams bytes directly into the HTTP response via `io.Pipe` — the client starts receiving data before the query finishes.

A single `errgroup` spans all stages. If anything fails, the shared context is cancelled immediately, every goroutine unblocks, and the error reaches the client. No leaks.

---

## Stack

| | |
|---|---|
| **HTTP** | [Fiber v3](https://github.com/gofiber/fiber) |
| **Database** | MySQL via [sqlx](https://github.com/jmoiron/sqlx) |
| **Concurrency** | [`errgroup`](https://pkg.go.dev/golang.org/x/sync/errgroup) + [`go-typedpipe`](https://github.com/fikrimohammad/go-typedpipe) |
| **Config** | YAML |

---

## Getting started

**1. Configure the database**

Copy the config template and fill in your credentials:

```bash
cp files/config/secret.yaml.example files/config/secret.yaml
```

```yaml
# files/config/secret.yaml
db:
  host: localhost
  port: 3306
  name: efficient_report_exporter
  username: your_user
  password: your_password
```

> ⚠️ Never commit `secret.yaml`. Add it to `.gitignore`.

**2. Run**

```bash
go run ./cmd
# Server listening on :3000
```

---

## API

### `POST /v1/reports/export`

Streams a CSV of all fee line items for a shop within a settlement date range.

**Request** (`application/x-www-form-urlencoded`)

| Field | Type | Format | Required |
|---|---|---|---|
| `shop_id` | int64 | — | ✓ |
| `start_time` | string | RFC3339Nano | ✓ |
| `end_time` | string | RFC3339Nano | ✓ |

**Example**

```bash
curl -X POST http://localhost:3000/v1/reports/export \
  -F "shop_id=1001" \
  -F "start_time=2024-01-01T00:00:00Z" \
  -F "end_time=2024-01-31T23:59:59Z" \
  --output report.csv
```

**Response**

```
Content-Disposition: attachment; filename="1001_20240101_20240131.csv"
Content-Type: application/octet-stream
```

```csv
Shop ID,Fee ID,Order ID,Order Creation Time,Order Payment Time,Order Settlement Time,Order Detail ID,Product ID,Category ID,Product Price Amount,Promo Amount,Fee Base Amount,Fee Final Amount
1001,42,9900001,2024-01-03 08:00:00,2024-01-03 08:01:22,2024-01-04 00:00:00,77001,301,5,150000.00,5000.00,10000.00,9500.00
...
```

---

## Project layout

```
.
├── cmd/                    # main, server bootstrap, config loader
├── handler/report/         # HTTP layer — parse request, stream response
├── usecase/report/         # pipeline orchestration (fetch → flatten → CSV)
├── repository/mysql/report/# async MySQL query + query builder
├── model/                  # Report, ReportLine, ReportFeeDetail types
└── files/config/           # secret.yaml (gitignored)
```
