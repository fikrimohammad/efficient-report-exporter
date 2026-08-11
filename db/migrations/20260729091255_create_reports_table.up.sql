CREATE TABLE IF NOT EXISTS report (
    id                    BIGINT          NOT NULL AUTO_INCREMENT,
    shop_id               BIGINT          NOT NULL,
    order_id              BIGINT          NOT NULL,
    order_creation_time   DATETIME        NOT NULL,
    order_payment_time    DATETIME        NOT NULL,
    order_settlement_time DATETIME        NOT NULL,
    fee_id                BIGINT          NOT NULL,
    details               JSON            NOT NULL,
    creation_time         DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    update_time           DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX idx_shop_settlement_id (shop_id, order_settlement_time, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
