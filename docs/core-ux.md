# Core UX — Report Export

The end-to-end user journey: the client makes **one** export request, then polls for
completion and downloads the result. All the heavy lifting happens out-of-band, so the
client never blocks on report generation.

```mermaid
flowchart LR
    C["Client"]:::client

    subgraph svc["Report Export Service"]
        direction LR
        API["API Service<br/>(Hertz)"]:::service
        MQ[["Message Queue<br/>(RocketMQ)"]]:::queue
        W["Worker<br/>(MQ Consumer)"]:::service
        DB[("MySQL")]:::db
    end

    S3[("Object Storage<br/>(S3)")]:::storage

    C <-->|"1. Request exporting report<br/>(POST /v1/reports/export)<br/>→ job_id"| API
    API -.->|"Process report asynchronously<br/>(topic: reporting · tag: export_report_process)"| MQ
    MQ -.->|"consume"| W
    W -.->|"read rows"| DB
    W -.->|"upload CSV / ZIP"| S3
    C <-->|"2. Poll exporting report status<br/>(GET /v1/reports/export/:job_id)<br/>→ status + presigned URL"| API
    C <-->|"3. Download report file<br/>(GET presigned URL)<br/>→ report file (CSV / ZIP)"| S3

    classDef client fill:#D6F5E3,stroke:#2BA84A,color:#14532D,stroke-width:2px
    classDef service fill:#DDEBFA,stroke:#2E7BD6,color:#1E3A5F,stroke-width:2px
    classDef queue fill:#FFF3CD,stroke:#E6A817,color:#6B5100,stroke-width:2px
    classDef db fill:#FDE2D3,stroke:#E05B3D,color:#6B2B15,stroke-width:2px
    classDef storage fill:#EADCF7,stroke:#8A4FC4,color:#3D2260,stroke-width:2px

    style svc fill:#fafafa,stroke:#94a3b8,stroke-dasharray: 5 5
```

Legend: **solid arrows** = synchronous call/response, **dashed arrows** = asynchronous
(event-driven, background).

| Step | Actor | Business process | Sync / Async |
| --- | --- | --- | --- |
| 1 | Client | Request exporting report (`POST /v1/reports/export`) — receives `job_id` immediately | Sync (fast) |
| — | Service | Process report asynchronously (publish to topic `reporting` / tag `export_report_process`): message → consumer → read rows → upload to S3 → mark job `success` | Async (background) |
| 2 | Client | Poll exporting report status (`GET /v1/reports/export/:job_id`) — receives `download_url` (presigned) once `success` | Sync |
| 3 | Client | Download report file (`GET` presigned URL) | Sync |

Key idea: from the client's point of view the flow is only three synchronous steps
(request → poll → download). The asynchronous processing runs behind the scenes.
