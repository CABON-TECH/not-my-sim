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
// If DEMO_ALERT_PHONE is set, the call is redirected there instead of the victim's number.
func InitiateRobocall(victimPhone string) {
	username := os.Getenv("AT_USERNAME")
	apiKey := os.Getenv("AT_API_KEY")
	fromNumber := os.Getenv("AT_VOICE_NUMBER")

	// Redirect to demo phone if configured — same pattern as Send2FASMS.
	callTo := victimPhone
	if demoPhone := os.Getenv("DEMO_ALERT_PHONE"); demoPhone != "" {
		log.Printf("📲 [DEMO MODE] Redirecting robocall from %s → %s", victimPhone, demoPhone)
		callTo = demoPhone
	}

	if username == "" || apiKey == "" || fromNumber == "" {
		log.Println("======================================================")
		log.Printf(" 📞 [VOICE SYSTEM] INITIATING EMERGENCY ROBOCALL TO: %s", callTo)
		log.Println(" 🤖 [VOICE SYSTEM] Playing automated fraud alert message...")
		log.Println("======================================================")
		return
	}

	// SAFETY GUARD: Only call whitelisted numbers in production.
	whitelist := os.Getenv("AT_WHITELIST")
	if whitelist != "" {
		allowed := false
		for _, w := range strings.Split(whitelist, ",") {
			if strings.TrimSpace(w) == callTo {
				allowed = true
				break
			}
		}
		if !allowed {
			log.Printf("🛡️  [VOICE BLOCKED] %s is not in AT_WHITELIST — skipping live call (safe mock only)", callTo)
			return
		}
	}

	endpoint := "https://voice.africastalking.com/call"
	if username == "sandbox" {
		endpoint = "https://voice.sandbox.africastalking.com/call"
	}

	data := url.Values{}
	data.Set("username", username)
	data.Set("to", callTo)
	data.Set("from", fromNumber)

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
		log.Printf("📞 ROBOCALL DISPATCHED | to=%s (Status %d)", callTo, resp.StatusCode)
	} else if resp.StatusCode == 401 {
		log.Printf("⚠️  [VOICE] Auth failed (401) — Voice service not yet activated on this AT account. SMS alert was still sent.")
	} else {
		var respBody map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&respBody)
		log.Printf("actions/voice: API returned %d: %v", resp.StatusCode, respBody)
	}
}
