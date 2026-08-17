-- Reverse convert_report_times_to_unix_milli: BIGINT unix milliseconds back to
-- DATETIME, restoring the CURRENT_TIMESTAMP defaults. The index is dropped up
-- front for the same reason as the up migration (see its comment).

ALTER TABLE report DROP INDEX idx_shop_settlement_id;

ALTER TABLE report
    ADD COLUMN order_creation_time_dt DATETIME NOT NULL DEFAULT '1970-01-01 00:00:00',
    ADD COLUMN order_payment_time_dt DATETIME NOT NULL DEFAULT '1970-01-01 00:00:00',
    ADD COLUMN order_settlement_time_dt DATETIME NOT NULL DEFAULT '1970-01-01 00:00:00',
    ADD COLUMN creation_time_dt DATETIME NOT NULL DEFAULT '1970-01-01 00:00:00',
    ADD COLUMN update_time_dt DATETIME NOT NULL DEFAULT '1970-01-01 00:00:00';

UPDATE report
SET order_creation_time_dt   = FROM_UNIXTIME(order_creation_time / 1000),
    order_payment_time_dt    = FROM_UNIXTIME(order_payment_time / 1000),
    order_settlement_time_dt = FROM_UNIXTIME(order_settlement_time / 1000),
    creation_time_dt         = FROM_UNIXTIME(creation_time / 1000),
    update_time_dt           = FROM_UNIXTIME(update_time / 1000);

ALTER TABLE report
    DROP COLUMN order_creation_time,
    DROP COLUMN order_payment_time,
    DROP COLUMN order_settlement_time,
    DROP COLUMN creation_time,
    DROP COLUMN update_time,
    RENAME COLUMN order_creation_time_dt TO order_creation_time,
    RENAME COLUMN order_payment_time_dt TO order_payment_time,
    RENAME COLUMN order_settlement_time_dt TO order_settlement_time,
    RENAME COLUMN creation_time_dt TO creation_time,
    RENAME COLUMN update_time_dt TO update_time,
    MODIFY COLUMN creation_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    MODIFY COLUMN update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    ADD INDEX idx_shop_settlement_id (shop_id, order_settlement_time, id);