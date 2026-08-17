-- Convert the report table's time columns from DATETIME to BIGINT unix
-- milliseconds, matching export_report_job. Temp columns hold the converted
-- values because MySQL cannot reinterpret a DATETIME value as an integer (or
-- vice versa) in place. The index is dropped up front: dropping the old
-- settlement column only removes it from the composite index (shop_id and id
-- remain), so ADD INDEX at the end would otherwise collide with the surviving
-- index of the same name.

ALTER TABLE report DROP INDEX idx_shop_settlement_id;

ALTER TABLE report
    ADD COLUMN order_creation_time_ms BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN order_payment_time_ms BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN order_settlement_time_ms BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN creation_time_ms BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN update_time_ms BIGINT NOT NULL DEFAULT 0;

UPDATE report
SET order_creation_time_ms   = UNIX_TIMESTAMP(order_creation_time) * 1000,
    order_payment_time_ms    = UNIX_TIMESTAMP(order_payment_time) * 1000,
    order_settlement_time_ms = UNIX_TIMESTAMP(order_settlement_time) * 1000,
    creation_time_ms         = UNIX_TIMESTAMP(creation_time) * 1000,
    update_time_ms           = UNIX_TIMESTAMP(update_time) * 1000;

ALTER TABLE report
    DROP COLUMN order_creation_time,
    DROP COLUMN order_payment_time,
    DROP COLUMN order_settlement_time,
    DROP COLUMN creation_time,
    DROP COLUMN update_time,
    RENAME COLUMN order_creation_time_ms TO order_creation_time,
    RENAME COLUMN order_payment_time_ms TO order_payment_time,
    RENAME COLUMN order_settlement_time_ms TO order_settlement_time,
    RENAME COLUMN creation_time_ms TO creation_time,
    RENAME COLUMN update_time_ms TO update_time,
    ADD INDEX idx_shop_settlement_id (shop_id, order_settlement_time, id);