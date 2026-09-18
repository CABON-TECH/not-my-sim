-- Migration 003: Graph edge table
-- Production equivalent: Neo4j nodes/edges with Louvain community detection.
-- Hackathon build: Postgres weighted edge table + SQL hub-scoring query.
-- Same conceptual idea — the structural pattern (shared recipients) is visible
-- in both; the query is just SQL GROUP BY instead of a graph algorithm.

CREATE TABLE IF NOT EXISTS graph_edges (
    id          BIGSERIAL    PRIMARY KEY,
    from_number TEXT         NOT NULL, -- victim phone number
    to_number   TEXT         NOT NULL, -- recipient / mule phone number
    weight      INT          NOT NULL DEFAULT 1, -- number of fraud-chain transfers on this edge
    last_seen   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (from_number, to_number)
);

-- Index for the hub-scoring query (range scan on last_seen, group by to_number).
CREATE INDEX IF NOT EXISTS idx_graph_edges_last_seen ON graph_edges (last_seen DESC);
CREATE INDEX IF NOT EXISTS idx_graph_edges_to_number ON graph_edges (to_number);
