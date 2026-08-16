namespace go mq

// ExportReportProcessMessage is the payload published on topic "reporting"
// with tag "export_report_process". It triggers the asynchronous processing
// of a report export job. Consumed by the export consumer and any other
// domain that needs to react to an export being requested.
//
// job_id is serialized as a string because it is a snowflake int64 that can
// exceed the JavaScript safe-integer range (2^53).
struct ExportReportProcessMessage {
    1: string job_id
}

// ExportReportDoneMessage is the payload published on topic "reporting" with
// tag "export_report_done". It signals that a report export job completed
// successfully, for downstream domains (e.g. notification services).
struct ExportReportDoneMessage {
    1: string job_id
}
