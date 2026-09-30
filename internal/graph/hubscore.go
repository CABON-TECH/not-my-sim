package graph

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

type HubResult struct {
	Recipient   string    `json:"recipient"`
	VictimCount int       `json:"victimCount"`  
	TotalWeight int       `json:"totalWeight"`  
	LastSeen    time.Time `json:"lastSeen"`
	RiskLabel string `json:"riskLabel"`
}


func QueryHubScore(db *sql.DB, minVictims, windowHours int) ([]HubResult, error) {
	rows, err := db.Query(`
		SELECT
		    to_number,
		    COUNT(DISTINCT from_number)  AS victim_count,
		    SUM(weight)                  AS total_weight,
		    MAX(last_seen)               AS last_seen
		FROM  graph_edges
		WHERE last_seen >= NOW() - ($1 * INTERVAL '1 hour')
		GROUP BY to_number
		HAVING COUNT(DISTINCT from_number) >= $2
		ORDER BY victim_count DESC, total_weight DESC`,
		windowHours, minVictims,
	)
	if err != nil {
		return nil, fmt.Errorf("graph: hub-score query: %w", err)
	}
	defer rows.Close()

	var results []HubResult
	for rows.Next() {
		var r HubResult
		if err := rows.Scan(&r.Recipient, &r.VictimCount, &r.TotalWeight, &r.LastSeen); err != nil {
			return nil, fmt.Errorf("graph: hub-score scan: %w", err)
		}
		r.RiskLabel = riskLabel(r.VictimCount)
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("graph: hub-score rows: %w", err)
	}
	return results, nil
}


func riskLabel(victimCount int) string {
	switch {
	case victimCount >= 5:
		return "CONFIRMED_SYNDICATE_HUB"
	case victimCount >= 3:
		return "PROBABLE_SYNDICATE_HUB"
	case victimCount >= 2:
		return "SYNDICATE_CANDIDATE"
	default:
		return "ISOLATED"
	}
}

func QueryDeviceCollusion(db *sql.DB, deviceID string) (int, error) {
	if deviceID == "" {
		return 1, nil
	}
	var count int
	err := db.QueryRow(`
		SELECT COUNT(DISTINCT phone_number) 
		FROM device_edges 
		WHERE device_id = $1`, deviceID).Scan(&count)
	if err != nil {
		return 1, fmt.Errorf("graph: QueryDeviceCollusion err: %w", err)
	}
	return count, nil
}

func HandleHubScore(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		minVictims := queryInt(r, "min_victims", 2)
		windowHours := queryInt(r, "window_hours", 24)

		results, err := QueryHubScore(db, minVictims, windowHours)
		if err != nil {
			log.Printf("graph: hub-score handler err: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if results == nil {
			results = []HubResult{} 
		}

		log.Printf("🕸️  Hub-score query | minVictims=%d windowHours=%d → %d flagged recipients",
			minVictims, windowHours, len(results))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"flaggedRecipients": results,
			"count":             len(results),
			"queryParams": map[string]int{
				"minVictims":  minVictims,
				"windowHours": windowHours,
			},
			"note": "Recipients linked to ≥2 distinct post-swap fraud chains — probable syndicate infrastructure",
		})
	}
}

func queryInt(r *http.Request, key string, def int) int {
	if v := r.URL.Query().Get(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}
