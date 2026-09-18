-- Migration 001: Initial schema
-- Creates the accounts table to confirm DB connectivity end-to-end (Phase 1 gate check).
-- Subsequent migrations add event and state tables.

CREATE TABLE IF NOT EXISTS accounts (
    phone_number  TEXT        PRIMARY KEY,           -- E.164 format, e.g. +254712345678
    registered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    agent_id      TEXT,                              -- telecom agent who last handled registration
    notes         TEXT
);

-- Seed a couple of known-good test accounts for the demo.
-- Real accounts will be created via the API during actual use.
INSERT INTO accounts (phone_number, agent_id, notes)
VALUES
    ('+254700000001', 'AGENT001', 'Demo victim account A'),
    ('+254700000002', 'AGENT001', 'Demo victim account B'),
    ('+254700000003', 'AGENT002', 'Demo mule / recipient account')
ON CONFLICT (phone_number) DO NOTHING;
