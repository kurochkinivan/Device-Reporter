BEGIN;

CREATE TYPE files_status AS ENUM (
    'pending',
    'processing',
    'done',
    'error'
);

CREATE TABLE IF NOT EXISTS files (
    name          TEXT        PRIMARY KEY,
    status        files_status NOT NULL DEFAULT 'pending',
    error_message TEXT        CHECK (error_message IS NULL OR error_message = '' OR status = 'error'),
    processed_at  TIMESTAMPTZ CHECK (processed_at IS NULL OR status IN ('done', 'error'))
);

CREATE TABLE IF NOT EXISTS devices (
    id         BIGSERIAL   PRIMARY KEY,
    n          INTEGER,
    mqtt       TEXT,
    inv_id     TEXT,
    unit_guid  UUID,
    msg_id     TEXT,
    text       TEXT,
    context    TEXT,
    class      TEXT,
    level      INTEGER,
    area       TEXT,
    addr       TEXT,
    block      TEXT,
    type       TEXT,
    bit        TEXT,
    invert_bit TEXT,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_devices_unit_guid ON devices(unit_guid);

COMMIT;