// Package graph implements the simplified graph layer described in design doc §3.4.
//
// Production equivalent: Neo4j nodes/edges, Louvain community detection,
// betweenness/eigenvector centrality queries.
//
// Hackathon build: Postgres weighted edge table + SQL GROUP BY hub-scoring query.
// The structural insight is identical — shared recipients form dense subgraphs
// that are structurally invisible in flat transaction tables but obvious once
// modeled as nodes and edges (or in our case, as an aggregated edge table).
package graph

import (
	"database/sql"
	"log"
	"time"
)

// WriteEdge upserts a directed fraud-chain edge: victim → recipient.
// Called by the state machine's onCritical hook whenever TRANSFER_POST_RESET fires.
// Weight increments on conflict so repeated transfers on the same victim→recipient
// path increase the edge weight — a proxy for confidence.
func WriteEdge(db *sql.DB, fromNumber, toNumber string) error {
	_, err := db.Exec(`
		INSERT INTO graph_edges (from_number, to_number, weight, last_seen)
		VALUES ($1, $2, 1, $3)
		ON CONFLICT (from_number, to_number) DO UPDATE SET
		    weight    = graph_edges.weight + 1,
		    last_seen = EXCLUDED.last_seen`,
		fromNumber, toNumber, time.Now().UTC(),
	)
	if err != nil {
		log.Printf("graph: WriteEdge from=%s to=%s err=%v", fromNumber, toNumber, err)
		return err
	}
	log.Printf("🕸️  Graph edge written | %s → %s (fraud chain confirmed)", fromNumber, toNumber)
	return nil
}

// WriteDeviceEdge links a phone number to a physical device ID.
func WriteDeviceEdge(db *sql.DB, phoneNumber, deviceID string) error {
	if deviceID == "" {
		return nil
	}
	_, err := db.Exec(`
		INSERT INTO device_edges (phone_number, device_id, last_seen)
		VALUES ($1, $2, $3)
		ON CONFLICT (phone_number, device_id) DO UPDATE SET
		    last_seen = EXCLUDED.last_seen`,
		phoneNumber, deviceID, time.Now().UTC(),
	)
	if err != nil {
		log.Printf("graph: WriteDeviceEdge err=%v", err)
		return err
	}
	return nil
}
