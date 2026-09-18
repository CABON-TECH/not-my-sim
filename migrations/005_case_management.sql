-- Migration 005: Case Management and Geo-Location tracking
-- Adds freeze controls to account_state and locations to events

ALTER TABLE account_state ADD COLUMN IF NOT EXISTS is_frozen BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE reset_events ADD COLUMN IF NOT EXISTS location TEXT DEFAULT '';
ALTER TABLE transfer_events ADD COLUMN IF NOT EXISTS location TEXT DEFAULT '';
