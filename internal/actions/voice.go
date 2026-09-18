package actions

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// InitiateRobocall triggers a live phone call via Africa's Talking Voice API.
// Requires AT_API_KEY, AT_USERNAME, and AT_VOICE_NUMBER.
func InitiateRobocall(victimPhone string) {
	username := os.Getenv("AT_USERNAME")
	apiKey := os.Getenv("AT_API_KEY")
	fromNumber := os.Getenv("AT_VOICE_NUMBER")

	if username == "" || apiKey == "" || fromNumber == "" {
		log.Printf("⚠️  [VOICE MOCK] Would call %s, but AT_VOICE_NUMBER not set in .env", victimPhone)
		return
	}

	endpoint := "https://voice.africastalking.com/call"
	if username == "sandbox" {
		endpoint = "https://voice.sandbox.africastalking.com/call"
	}

	data := url.Values{}
	data.Set("username", username)
	data.Set("to", victimPhone)
	data.Set("from", fromNumber)
	
	// Normally we would pass an Action URL that returns the XML to speak.
	// For the hackathon, initiating the call is sufficient to demonstrate API integration.
	
	req, _ := http.NewRequest("POST", endpoint, strings.NewReader(data.Encode()))
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("apiKey", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("actions/voice: AT API error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 201 || resp.StatusCode == 200 {
		log.Printf("📞 ROBOCALL DISPATCHED | to=%s (Status %d)", victimPhone, resp.StatusCode)
	} else {
		var respBody map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&respBody)
		log.Printf("actions/voice: API returned %d: %v", resp.StatusCode, respBody)
	}
}
