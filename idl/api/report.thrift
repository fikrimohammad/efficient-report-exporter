namespace go api

include "base.thrift"

struct ExportReportRequest {
    1: string request_id (api.body="request_id")
    2: string shop_id    (api.body="shop_id")
    3: string start_time (api.body="start_time")
    4: string end_time   (api.body="end_time")
}

struct ExportReportData {
    1: string job_id (api.body="job_id")
}

struct ExportReportResponse {
    1: base.BaseResponse base (api.body="base")
    2: ExportReportData  data  (api.body="data")
}

struct GetExportReportJobRequest {
    1: string job_id (api.path="job_id")
}

struct GetExportReportJobData {
    1: string  job_id        (api.body="job_id")
    2: string  status        (api.body="status")
    3: string  download_url  (api.body="download_url")
    4: string  error_message (api.body="error_message")
    5: string  created_at    (api.body="created_at")
    6: string  updated_at    (api.body="updated_at")
}

struct GetExportReportJobResponse {
    1: base.BaseResponse       base (api.body="base")
    2: GetExportReportJobData  data (api.body="data")
}

struct ListExportReportJobsRequest {
    1: string shop_id     (api.query="shop_id")
    2: string page_token  (api.query="page_token")
    3: string limit       (api.query="limit")
}

struct ExportReportJobSummary {
    1: string job_id       (api.body="job_id")
    2: string status       (api.body="status")
    3: string start_time   (api.body="start_time")
    4: string end_time     (api.body="end_time")
    5: string created_at   (api.body="created_at")
    6: string updated_at   (api.body="updated_at")
}

struct ListExportReportJobsData {
    1: list<ExportReportJobSummary> jobs            (api.body="jobs")
    2: string                       next_page_token (api.body="next_page_token")
}

struct ListExportReportJobsResponse {
    1: base.BaseResponse          base (api.body="base")
    2: ListExportReportJobsData   data (api.body="data")
}

service ReportService {
    ExportReportResponse ExportReport(1: ExportReportRequest req) (api.post="/v1/reports/export");
    GetExportReportJobResponse GetExportReportJob(1: GetExportReportJobRequest req) (api.get="/v1/reports/export/:job_id");
    ListExportReportJobsResponse ListExportReportJobs(1: ListExportReportJobsRequest req) (api.get="/v1/reports/export");
}
