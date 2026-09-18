package actions

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type SMSState struct {
	To      string
	Message string
	Time    string
}

// LastSMS holds the most recently dispatched SMS
var LastSMS SMSState

// Send2FASMS sends an interactive SMS via Africa's Talking.
func Send2FASMS(victimPhone string, message string) {
	LastSMS = SMSState{
		To:      victimPhone,
		Message: message,
		Time:    time.Now().Format("15:04"),
	}

	username := os.Getenv("AT_USERNAME")
	apiKey := os.Getenv("AT_API_KEY")
	fromNumber := os.Getenv("AT_SMS_SHORTCODE") // e.g. "20880"

	if username == "" || apiKey == "" {
		log.Printf("⚠️  [SMS MOCK] Would send: '%s' to %s (AT keys missing)", message, victimPhone)
		return
	}
	if fromNumber == "" {
		fromNumber = "NotMySim" // Default alphanumeric
	}

	endpoint := "https://api.africastalking.com/version1/messaging"
	if username == "sandbox" {
		endpoint = "https://api.sandbox.africastalking.com/version1/messaging"
	}

	data := url.Values{}
	data.Set("username", username)
	data.Set("to", victimPhone)
	data.Set("message", message)
	data.Set("from", fromNumber)

	req, _ := http.NewRequest("POST", endpoint, strings.NewReader(data.Encode()))
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("apiKey", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("actions/sms: AT API error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 201 || resp.StatusCode == 200 {
		log.Printf("📩 2FA SMS DISPATCHED | to=%s (Status %d)", victimPhone, resp.StatusCode)
	} else {
		var respBody map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&respBody)
		log.Printf("actions/sms: API returned %d: %v", resp.StatusCode, respBody)
	}
}

// SendFraudAlert is a mock for sending transaction-held alerts to the victim.
func SendFraudAlert(atClient interface{}, fromNumber, toNumber string, amount float64, score int) {
	log.Printf("📱 [SMS MOCK] FRAUD ALERT SENT | from=%s to=%s amount=%.2f score=%d", fromNumber, toNumber, amount, score)
}
