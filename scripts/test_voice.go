//go:build ignore

// Quick test to verify the Africa's Talking Voice API is working.
// Usage:
//
//	go run scripts/test_voice.go
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func main() {
	username := os.Getenv("AT_USERNAME")
	apiKey   := os.Getenv("AT_API_KEY")
	fromNumber := os.Getenv("AT_VOICE_NUMBER")
	toNumber   := os.Getenv("DEMO_ALERT_PHONE")

	fmt.Println("=== Africa's Talking Voice API Test ===")
	fmt.Printf("  Username:     %s\n", username)
	fmt.Printf("  From (voice): %s\n", fromNumber)
	fmt.Printf("  To (calling): %s\n", toNumber)
	fmt.Println()

	if username == "" || apiKey == "" {
		log.Fatal("❌ AT_USERNAME or AT_API_KEY not set")
	}
	if fromNumber == "" {
		log.Fatal("❌ AT_VOICE_NUMBER not set — go to AT dashboard → Voice → Phone Numbers")
	}
	if toNumber == "" {
		log.Fatal("❌ DEMO_ALERT_PHONE not set")
	}

	endpoint := "https://voice.africastalking.com/call"
	if username == "sandbox" {
		endpoint = "https://voice.sandbox.africastalking.com/call"
	}
	fmt.Printf("  Endpoint: %s\n\n", endpoint)

	data := url.Values{}
	data.Set("username", username)
	data.Set("to", toNumber)
	data.Set("from", fromNumber)

	req, _ := http.NewRequest("POST", endpoint, strings.NewReader(data.Encode()))
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("apiKey", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("❌ HTTP error: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var pretty map[string]interface{}
	json.Unmarshal(body, &pretty)

	fmt.Printf("  Status:   %d\n", resp.StatusCode)
	fmt.Printf("  Response: %s\n\n", string(body))

	switch resp.StatusCode {
	case 200, 201:
		fmt.Println("✅ SUCCESS — Your phone should be ringing now!")
	case 401:
		fmt.Println("❌ 401 Auth Failed — Voice service NOT activated on your AT account.")
		fmt.Println("   → Go to account.africastalking.com → Voice → activate the service")
		fmt.Println("   → Check AT_VOICE_NUMBER matches a number in your AT Voice dashboard")
	case 400:
		fmt.Println("❌ 400 Bad Request — AT_VOICE_NUMBER is wrong or not provisioned.")
		fmt.Println("   → Go to AT dashboard → Voice → Phone Numbers → copy your exact number")
	default:
		fmt.Printf("❌ Unexpected status %d\n", resp.StatusCode)
	}
}
