-- Migration 002: Events + State Machine tables

-- Swap events received from AT webhook
CREATE TABLE IF NOT EXISTS swap_events (
    id           BIGSERIAL    PRIMARY KEY,
    phone_number TEXT         NOT NULL,
    request_id   TEXT         NOT NULL UNIQUE, -- AT's requestId, idempotency key
    status       TEXT         NOT NULL,        -- "Swapped", "NotSwapped", etc.
    swap_date_str TEXT,                        -- "DD-MM-YYYY" string from AT
    received_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- PIN / password reset attempt events
CREATE TABLE IF NOT EXISTS reset_events (
    id           BIGSERIAL    PRIMARY KEY,
    phone_number TEXT         NOT NULL,
    event_id     TEXT         NOT NULL UNIQUE, -- caller-supplied idempotency key
    occurred_at  TIMESTAMPTZ  NOT NULL,
    received_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Money transfer events
CREATE TABLE IF NOT EXISTS transfer_events (
    id             BIGSERIAL    PRIMARY KEY,
    transaction_id TEXT         NOT NULL UNIQUE, -- caller-supplied idempotency key
    from_number    TEXT         NOT NULL,
    to_number      TEXT         NOT NULL,
    amount         NUMERIC(15,2) NOT NULL,
    occurred_at    TIMESTAMPTZ  NOT NULL,
    received_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- One row per account — the live state machine state, persisted after every transition.
-- No FK to accounts so the state machine can handle any phone number, not just seeded ones.
CREATE TABLE IF NOT EXISTS account_state (
    phone_number           TEXT         PRIMARY KEY,
    state                  TEXT         NOT NULL DEFAULT 'NORMAL',
    state_changed_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    risk_window_expires_at TIMESTAMPTZ,          -- NULL when in NORMAL state
    last_swap_request_id   TEXT                  -- AT requestId of the triggering swap
);

-- Correlates AT requestId → phone number for async webhook lookup.
-- Written when InvokeSimSwapCheck is called; read when the status webhook fires.
CREATE TABLE IF NOT EXISTS pending_checks (
    request_id   TEXT         PRIMARY KEY,
    phone_number TEXT         NOT NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Pre-seed state rows for the demo accounts so lookups never miss.
INSERT INTO account_state (phone_number, state)
VALUES
    ('+254700000001', 'NORMAL'),
    ('+254700000002', 'NORMAL'),
    ('+254700000003', 'NORMAL')
ON CONFLICT (phone_number) DO NOTHING;
