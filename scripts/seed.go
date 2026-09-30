//go:build ignore
// +build ignore

// Seed script for the not-my-sim demo.
// Simulates 2 victims (Account A and Account B) both going through the full
// fraud chain and converging on the same recipient (Recipient X).
//
// Usage:
//
//	go run scripts/seed.go              # uses http://localhost:8089
//	SERVER_URL=http://localhost:8080 go run scripts/seed.go
//	go run scripts/seed.go --reset-only # just reset state, don't seed
//
// Expected outcome after running:
//
//	GET /graph/hub-score returns +254700000003 with victimCount=2, riskLabel=SYNDICATE_CANDIDATE
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"
)

var baseURL string

func main() {
	resetOnly := flag.Bool("reset-only", false, "only reset state, skip seeding")
	flag.Parse()

	baseURL = os.Getenv("SERVER_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8089"
	}

	// Verify server is reachable.
	if _, err := http.Get(baseURL + "/health"); err != nil {
		log.Printf("Server healthcheck failed, but attempting to proceed...")
	}

	if *resetOnly {
		log.Println("\n--- Resetting demo state ---")
		post("/admin/reset-demo", nil)
		log.Println("✅ Reset complete")
		return
	}

	rand.Seed(time.Now().UnixNano())
	suffix := rand.Intn(900000) + 100000 // 6 digit random number

	phoneA := fmt.Sprintf("+254700%d", suffix)
	phoneB := fmt.Sprintf("+254701%d", suffix)
	
	reqIdA := fmt.Sprintf("ATSwpid_seed_A_%d", suffix)
	reqIdB := fmt.Sprintf("ATSwpid_seed_B_%d", suffix)
	resetIdA := fmt.Sprintf("reset-seed-A-%d", suffix)
	resetIdB := fmt.Sprintf("reset-seed-B-%d", suffix)
	txnIdA := fmt.Sprintf("txn-seed-A-%d", suffix)
	txnIdB := fmt.Sprintf("txn-seed-B-%d", suffix)

	log.Printf("🌱 Injecting attack simulation | Victims: %s, %s", phoneA, phoneB)

	// =========================================================
	// Account A → full fraud chain → Recipient X
	// =========================================================
	log.Printf("\n--- Attacking Account A: %s ---", phoneA)

	post("/webhooks/simswap", M{
		"status": "Swapped", "lastSimSwapDate": today(),
		"requestId": reqIdA, "phoneNumber": phoneA,
		"agentId": "agent-kpl-409",
	})
	pause()

	post("/events/reset", M{
		"eventId": resetIdA, "phoneNumber": phoneA,
		"deviceId": "imei-fraud-999", "location": "Nairobi, KE", 
		"occurredAt": nowRFC3339(),
	})
	pause()

	post("/events/transaction", M{
		"transactionId": txnIdA, "fromNumber": phoneA,
		"toNumber": "+254700000003", "amount": 50000,
		"deviceId": "imei-fraud-999", "location": "Mombasa, KE", 
		"occurredAt": nowRFC3339(),
	})
	time.Sleep(400 * time.Millisecond)

	// =========================================================
	// Account B → full fraud chain → same Recipient X
	// =========================================================
	log.Printf("\n--- Attacking Account B: %s ---", phoneB)

	post("/webhooks/simswap", M{
		"status": "Swapped", "lastSimSwapDate": today(),
		"requestId": reqIdB, "phoneNumber": phoneB,
		"agentId": "agent-kpl-409",
	})
	pause()

	post("/events/reset", M{
		"eventId": resetIdB, "phoneNumber": phoneB,
		"deviceId": "imei-fraud-999", "location": "Nairobi, KE",
		"occurredAt": nowRFC3339(),
	})
	pause()

	post("/events/transaction", M{
		"transactionId": txnIdB, "fromNumber": phoneB,
		"toNumber": "+254700000003", "amount": 75000,
		"deviceId": "imei-fraud-999", "location": "Mombasa, KE",
		"occurredAt": nowRFC3339(),
	})
	time.Sleep(400 * time.Millisecond)

	// =========================================================
	// Account C → Grey Area 2FA Trigger (70 Points)
	// =========================================================
	// Account C uses a different prefix so it's clearly distinct from A and B.
	// It is still random — no real number is ever hardcoded in this script.
	phoneC := fmt.Sprintf("+254729%d", suffix)
	log.Printf("\n--- Attacking Account C (Grey Area 2FA): %s ---", phoneC)

	// SIM Swap + Reset + Isolated Transfer
	post("/webhooks/simswap", M{
		"status": "Swapped", "lastSimSwapDate": today(),
		"requestId": "req-seed-C", "phoneNumber": phoneC,
		"agentId": "agent-kpl-409",
	})
	pause()

	post("/events/reset", M{
		"eventId": "reset-seed-C-999888", "phoneNumber": phoneC,
		"deviceId": "imei-suspicious-555", "location": "Kisumu, KE",
		"occurredAt": nowRFC3339(),
	})
	pause()

	post("/events/transaction", M{
		"transactionId": "txn-seed-C-999888", "fromNumber": phoneC,
		"toNumber": "+254700000004", "amount": 80000,
		"deviceId": "imei-suspicious-555", "location": "Kisumu, KE",
		"occurredAt": nowRFC3339(),
	})
	time.Sleep(400 * time.Millisecond)

	log.Printf("✅ SIMULATION COMPLETE\n")
}

// M is a shorthand for map[string]interface{}.
type M = map[string]interface{}

func post(path string, body M) {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	resp, err := http.Post(baseURL+path, "application/json", &buf)
	if err != nil {
		log.Printf("  ❌ POST %s failed: %v", path, err)
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("  ← %d %s", resp.StatusCode, string(bytes.TrimSpace(respBody)))
}

func pause() { time.Sleep(150 * time.Millisecond) }

func today() string    { return time.Now().Format("02-01-2006") }
func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }
