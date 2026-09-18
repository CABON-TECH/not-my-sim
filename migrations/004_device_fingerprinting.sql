-- Migration 004: Device Fingerprinting
-- Tracks which physical devices are associated with which phone numbers.
-- If one device is linked to multiple compromised accounts, it signals agent/fraudster collusion.

CREATE TABLE IF NOT EXISTS device_edges (
    phone_number TEXT NOT NULL,
    device_id    TEXT NOT NULL,
    last_seen    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (phone_number, device_id)
);

CREATE INDEX IF NOT EXISTS idx_device_edges_device ON device_edges (device_id);
