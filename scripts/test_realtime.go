package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func post(path string, payload map[string]interface{}) {
	b, _ := json.Marshal(payload)
	resp, err := http.Post("http://localhost:8089"+path, "application/json", bytes.NewBuffer(b))
	if err != nil {
		log.Printf("   ❌ POST %s failed: %v", path, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("   ✓ OK")
	} else {
		log.Printf("   ❌ Failed with status: %d", resp.StatusCode)
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run scripts/test_realtime.go [sms|voice]")
		os.Exit(1)
	}

	phone := "+254729080194"
	mode := os.Args[1]

	log.Printf("=========================================================")
	log.Printf("🚀 STARTING REAL-TIME TEST FOR: %s", phone)
	log.Printf("=========================================================")

	if mode == "sms" {
		log.Println("\n[TEST: INTERACTIVE SMS (Score 70/100)]")
		log.Println("1. Injecting SIM Swap...")
		post("/webhooks/simswap", map[string]interface{}{
			"status": "Swapped", "lastSimSwapDate": time.Now().Format("02-01-2006"),
			"requestId": "req-test-sms", "phoneNumber": phone, "agentId": "agent-kpl-409",
		})
		time.Sleep(1 * time.Second)

		log.Println("2. Injecting PIN Reset...")
		post("/events/reset", map[string]interface{}{
			"eventId": "reset-test-sms", "phoneNumber": phone,
			"deviceId": "imei-test-1", "location": "Nairobi, KE",
			"occurredAt": time.Now().UTC().Format(time.RFC3339),
		})
		time.Sleep(1 * time.Second)

		log.Println("3. Injecting Isolated Transfer (This will trigger the SMS)...")
		post("/events/transaction", map[string]interface{}{
			"transactionId": "txn-test-sms", "fromNumber": phone,
			"toNumber": "+254799000111", "amount": 50000,
			"deviceId": "imei-test-1", "location": "Nairobi, KE", // Same location, no impossible travel
			"occurredAt": time.Now().UTC().Format(time.RFC3339),
		})
		log.Println("\n✅ DONE! Check your phone/simulator for the SMS.")
		log.Println("Reply '2' to the SMS to test the Webhook & Account Freeze.")

	} else if mode == "voice" {
		log.Println("\n[TEST: VOICE ROBOCALL (Score 100/100)]")
		log.Println("1. Injecting SIM Swap...")
		post("/webhooks/simswap", map[string]interface{}{
			"status": "Swapped", "lastSimSwapDate": time.Now().Format("02-01-2006"),
			"requestId": "req-test-voice", "phoneNumber": phone, "agentId": "agent-kpl-409",
		})
		time.Sleep(1 * time.Second)

		log.Println("2. Injecting PIN Reset in Nairobi...")
		post("/events/reset", map[string]interface{}{
			"eventId": "reset-test-voice", "phoneNumber": phone,
			"deviceId": "imei-test-2", "location": "Nairobi, KE",
			"occurredAt": time.Now().UTC().Format(time.RFC3339),
		})
		time.Sleep(1 * time.Second)

		log.Println("3. Injecting Impossible Travel Transfer from Mombasa (This will trigger the Call)...")
		post("/events/transaction", map[string]interface{}{
			"transactionId": "txn-test-voice", "fromNumber": phone,
			"toNumber": "+254799000222", "amount": 85000,
			"deviceId": "imei-test-2", "location": "Mombasa, KE", // Impossible travel triggers 100/100
			"occurredAt": time.Now().UTC().Format(time.RFC3339),
		})
		log.Println("\n✅ DONE! Check your phone/simulator for the Voice Call.")

	} else {
		fmt.Println("Invalid mode. Use 'sms' or 'voice'.")
	}
}
