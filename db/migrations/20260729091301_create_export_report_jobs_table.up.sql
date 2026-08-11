CREATE TABLE IF NOT EXISTS export_report_job (
    id              BIGINT          NOT NULL,
    request_id      BIGINT          NOT NULL,
    shop_id         BIGINT          NOT NULL,
    start_time      BIGINT          NOT NULL,
    end_time        BIGINT          NOT NULL,
    status          VARCHAR(20)     NOT NULL DEFAULT 'processing',
    extra           JSON            NOT NULL,
    creation_time   BIGINT          NOT NULL,
    update_time     BIGINT          DEFAULT NULL,
    PRIMARY KEY (id),
    INDEX idx_request_id (request_id),
    INDEX idx_shop_id (shop_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
