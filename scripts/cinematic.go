package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

const serverURL = "http://localhost:8089"

func main() {
	log.Println("🎬 STARTING CINEMATIC SIMULATION (SLOW MOTION)")
	log.Println("=========================================================")
	
	// Reset first just in case
	http.Post(serverURL+"/admin/reset-demo", "application/json", nil)
	time.Sleep(1 * time.Second)

	phoneA := "+254700111222"
	phoneB := "+254701111222"
	

	// --- ACCOUNT A ---
	log.Printf("\n[SCENE 1] Attacking Account A: %s", phoneA)
	log.Println("  -> Injecting SIM Swap (Compromising Telco Node)")
	post("/webhooks/simswap", map[string]interface{}{
		"status": "Swapped", "lastSimSwapDate": time.Now().Format("02-01-2006"),
		"requestId": "req-cin-A", "phoneNumber": phoneA, "agentId": "agent-kpl-409",
	})
	time.Sleep(3 * time.Second)

	log.Println("  -> Injecting PIN Reset from suspicious device (imei-fraud-999)")
	post("/events/reset", map[string]interface{}{
		"eventId": "reset-cin-A", "phoneNumber": phoneA,
		"deviceId": "imei-fraud-999", "location": "Nairobi, KE",
		"occurredAt": time.Now().UTC().Format(time.RFC3339),
	})
	time.Sleep(3 * time.Second)

	log.Println("  -> Injecting High-Risk Transfer to Mule (+254700000003)")
	post("/events/transaction", map[string]interface{}{
		"transactionId": "txn-cin-A", "fromNumber": phoneA,
		"toNumber": "+254700000003", "amount": 50000,
		"deviceId": "imei-fraud-999", "location": "Mombasa, KE",
		"occurredAt": time.Now().UTC().Format(time.RFC3339),
	})
	time.Sleep(4 * time.Second)

	// --- ACCOUNT B ---
	log.Printf("\n[SCENE 2] Attacking Account B: %s", phoneB)
	log.Println("  -> The Syndicate strikes again. SIM Swap initiated.")
	post("/webhooks/simswap", map[string]interface{}{
		"status": "Swapped", "lastSimSwapDate": time.Now().Format("02-01-2006"),
		"requestId": "req-cin-B", "phoneNumber": phoneB, "agentId": "agent-kpl-409",
	})
	time.Sleep(3 * time.Second)

	log.Println("  -> Pin Reset using the SAME hardware device (Device Collusion)")
	post("/events/reset", map[string]interface{}{
		"eventId": "reset-cin-B", "phoneNumber": phoneB,
		"deviceId": "imei-fraud-999", "location": "Nairobi, KE",
		"occurredAt": time.Now().UTC().Format(time.RFC3339),
	})
	time.Sleep(3 * time.Second)

	log.Println("  -> Transferring to the SAME Mule (Graph Hub Detected!)")
	post("/events/transaction", map[string]interface{}{
		"transactionId": "txn-cin-B", "fromNumber": phoneB,
		"toNumber": "+254700000003", "amount": 75000,
		"deviceId": "imei-fraud-999", "location": "Mombasa, KE",
		"occurredAt": time.Now().UTC().Format(time.RFC3339),
	})
	time.Sleep(4 * time.Second)

	// --- ACCOUNT C ---
	// Target user device for Interactive SMS
	phoneC := "+254119391977"
	log.Printf("\n[SCENE 3] The 'Grey Area' Attack on Account C: %s", phoneC)
	log.Println("  -> SIM Swap occurs, but attacker waits a while before acting.")
	post("/webhooks/simswap", map[string]interface{}{
		"status": "Swapped", "lastSimSwapDate": time.Now().Format("02-01-2006"),
		"requestId": "req-cin-C", "phoneNumber": phoneC, "agentId": "agent-kpl-409",
	})
	time.Sleep(3 * time.Second)

	log.Println("  -> Simulating anomaly: PIN reset from unrecognized location (Score: 70)")
	post("/events/reset", map[string]interface{}{
		"eventId": "reset-cin-C", "phoneNumber": phoneC,
		"deviceId": "imei-suspicious-555", "location": "Kisumu, KE",
		"occurredAt": time.Now().UTC().Format(time.RFC3339),
	})
	time.Sleep(3 * time.Second)

	post("/events/transaction", map[string]interface{}{
		"transactionId": "txn-cin-C", "fromNumber": phoneC,
		"toNumber": "+254700000004", "amount": 80000,
		"deviceId": "imei-suspicious-555", "location": "Kisumu, KE",
		"occurredAt": time.Now().UTC().Format(time.RFC3339),
	})
	
	log.Println("\n✅ CINEMATIC SIMULATION COMPLETE. Waiting for user to reply to 2FA SMS.")
}

func post(path string, payload map[string]interface{}) {
	b, _ := json.Marshal(payload)
	resp, err := http.Post(serverURL+path, "application/json", bytes.NewBuffer(b))
	if err != nil {
		fmt.Printf("   ❌ error: %v\n", err)
		return
	}
	resp.Body.Close()
	fmt.Printf("   ✓ OK\n")
}
