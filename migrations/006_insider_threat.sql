-- Migration 006: Insider Threat Detection
-- Adds agent tracking to SIM swap events to detect corrupt telecom employees

ALTER TABLE swap_events ADD COLUMN IF NOT EXISTS agent_id TEXT NOT NULL DEFAULT 'system';
