import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend } from 'k6/metrics';

// Load test for the report export flow: POST /v1/reports/export, then poll
// until the job reaches a terminal state. Each virtual user (VU) drives one
// logical export against a distinct seeded shop.
//
// Custom trends:
//   export_request  POST latency (ms)
//   export_process  poll-until-done latency (ms)
//   export_total    full POST -> success latency (ms)
//
// Usage (Docker, host networking so localhost reaches the API):
//   docker run --rm --network host -v "$PWD/k6:/scripts" \
//     -e API_URL=http://localhost:18081 \
//     grafana/k6 run --vus 8 --iterations 8 \
//     --summary-trend-stats="avg,min,med,p(90),p(99),max" /scripts/load.js

const API = __ENV.API_URL || 'http://localhost:18081';
const START_TIME = __ENV.START_TIME || '2026-08-01T00:00:00Z';
const END_TIME = __ENV.END_TIME || '2026-08-02T00:00:00Z';
const START_SHOP = parseInt(__ENV.START_SHOP || '500000', 10);
const SHOP_COUNT = parseInt(__ENV.SHOP_COUNT || '16', 10);
const POLL_INTERVAL = 0.1; // seconds
const POLL_DEADLINE = 120; // seconds

const exportRequest = new Trend('export_request', true);
const exportProcess = new Trend('export_process', true);
const exportTotal = new Trend('export_total', true);

export default function () {
  // Distinct shop per VU, cycling if there are more VUs than seeded shops.
  const shopID = START_SHOP + ((__VU - 1) % SHOP_COUNT);
  const requestID = `${Date.now()}${__VU}${__ITER}`;

  // 1. Request the export.
  const t0 = Date.now();
  const post = http.post(
    `${API}/v1/reports/export`,
    JSON.stringify({
      request_id: requestID,
      shop_id: String(shopID),
      start_time: START_TIME,
      end_time: END_TIME,
    }),
    { headers: { 'Content-Type': 'application/json' }, timeout: '30s' },
  );
  exportRequest.add(Date.now() - t0);

  let jobID = '';
  try {
    jobID = post.json('data.job_id') || '';
  } catch (_) {
    jobID = '';
  }
  check(post, { 'export POST 200': (r) => r.status === 200 && !!jobID });
  if (!jobID) return;

  // 2. Poll until the job reaches a terminal state.
  const procStart = Date.now();
  let status = 'processing';
  const deadline = Date.now() + POLL_DEADLINE * 1000;
  while (status === 'processing' && Date.now() < deadline) {
    const get = http.get(`${API}/v1/reports/export/${jobID}`, { timeout: '30s' });
    try {
      status = get.json('data.status') || 'processing';
    } catch (_) {
      status = 'processing';
    }
    if (status === 'processing') sleep(POLL_INTERVAL);
  }
  exportProcess.add(Date.now() - procStart);
  exportTotal.add(Date.now() - t0);
  check(status, { 'export success': (s) => s === 'success' });
}
