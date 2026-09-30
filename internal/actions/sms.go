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
// If DEMO_ALERT_PHONE is set, the SMS is redirected to that number instead of
// the victim's number. Use this during pitches/demos so no real random-generated
// number ever receives a message — the presenter's phone gets it instead.
func Send2FASMS(victimPhone string, message string) {
	// Redirect to demo phone if configured.
	sendTo := victimPhone
	if demoPhone := os.Getenv("DEMO_ALERT_PHONE"); demoPhone != "" {
		log.Printf("📲 [DEMO MODE] Redirecting 2FA SMS from %s → %s", victimPhone, demoPhone)
		sendTo = demoPhone
	}

	LastSMS = SMSState{
		To:      sendTo,
		Message: message,
		Time:    time.Now().Format("15:04"),
	}

	username := os.Getenv("AT_USERNAME")
	apiKey := os.Getenv("AT_API_KEY")
	fromNumber := os.Getenv("AT_SMS_SHORTCODE") // e.g. "20880"

	if username == "" || apiKey == "" {
		log.Printf("⚠️  [SMS MOCK] Would send: '%s' to %s (AT keys missing)", message, sendTo)
		return
	}

	// SAFETY GUARD: In production, only send to explicitly whitelisted numbers.
	// This prevents the demo from accidentally texting random real people.
	// Set AT_WHITELIST=+254729080194,+254711111111 in .env to allow specific numbers.
	// Leave AT_WHITELIST empty to allow ALL numbers (fully open - use with caution).
	whitelist := os.Getenv("AT_WHITELIST")
	if whitelist != "" {
		allowed := false
		for _, w := range strings.Split(whitelist, ",") {
			if strings.TrimSpace(w) == sendTo {
				allowed = true
				break
			}
		}
		if !allowed {
			log.Printf("🛡️  [SMS BLOCKED] %s is not in AT_WHITELIST — skipping live dispatch (safe mock only)", sendTo)
			return
		}
	}

	endpoint := "https://api.africastalking.com/version1/messaging"
	if username == "sandbox" {
		endpoint = "https://api.sandbox.africastalking.com/version1/messaging"
	}

	data := url.Values{}
	data.Set("username", username)
	data.Set("to", sendTo)
	data.Set("message", message)
	if fromNumber != "" {
		data.Set("from", fromNumber)
	}

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
		log.Printf("📩 2FA SMS DISPATCHED | to=%s (Status %d)", sendTo, resp.StatusCode)
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

// SendFreezeNotification tells a victim their account has been frozen by the system.
// Respects DEMO_ALERT_PHONE — in demo mode the SMS lands on the presenter's phone.
func SendFreezeNotification(victimPhone string) {
	msg := "NOT-MY-SIM ALERT: Your mobile money account has been FROZEN by our fraud protection system due to suspicious SIM swap activity. Contact your provider immediately to verify and restore access."
	Send2FASMS(victimPhone, msg)
}

// SendAnalystAlert fires a summary SMS to the ALERT_PHONE (analyst/admin) when a
// network takedown is executed. Set ALERT_PHONE in .env to activate.
func SendAnalystAlert(mule string, victims []string) {
	phone := os.Getenv("ALERT_PHONE")
	if phone == "" {
		log.Printf("📋 [ANALYST ALERT MOCK] Takedown executed | mule=%s victims=%v (set ALERT_PHONE in .env to receive live SMS)", mule, victims)
		return
	}
	msg := "NOT-MY-SIM: NETWORK NEUTRALIZED. Mule: " + mule +
		" | Victims frozen: " + strings.Join(victims, ", ") +
		" | Action taken at " + time.Now().Format("15:04:05")
	dispatchSMS(phone, msg)
	log.Printf("📟 ANALYST ALERT SENT | to=%s mule=%s victims=%v", phone, mule, victims)
}

// dispatchSMS is the shared low-level AT SMS sender used by all action functions.
// It does NOT apply the DEMO_ALERT_PHONE redirect — callers handle that themselves.
func dispatchSMS(to, message string) {
	username := os.Getenv("AT_USERNAME")
	apiKey := os.Getenv("AT_API_KEY")
	fromNumber := os.Getenv("AT_SMS_SHORTCODE")

	if username == "" || apiKey == "" {
		log.Printf("⚠️  [SMS MOCK] Would send: '%s' to %s (AT keys missing)", message, to)
		return
	}

	endpoint := "https://api.africastalking.com/version1/messaging"
	if username == "sandbox" {
		endpoint = "https://api.sandbox.africastalking.com/version1/messaging"
	}

	data := url.Values{}
	data.Set("username", username)
	data.Set("to", to)
	data.Set("message", message)
	if fromNumber != "" {
		data.Set("from", fromNumber)
	}

	req, _ := http.NewRequest("POST", endpoint, strings.NewReader(data.Encode()))
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("apiKey", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("actions/sms: dispatchSMS error: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		log.Printf("📩 SMS DISPATCHED | to=%s (Status %d)", to, resp.StatusCode)
	} else {
		var body map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&body)
		log.Printf("actions/sms: API returned %d: %v", resp.StatusCode, body)
	}
}
