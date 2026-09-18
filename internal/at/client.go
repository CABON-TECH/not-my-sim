// Package at provides a thin client for the Africa's Talking HTTP API.
// It covers the two endpoints this project uses:
//   - SIM Swap check invocation (async — result delivered via webhook)
//   - SMS sending
//
// The AT SIM Swap API is ASYNCHRONOUS:
//  1. You call GET /sim-swap/invoke-check-simswap?phone=<number>
//     → AT queues the check and returns {"status":"Queued","requestId":"..."}
//  2. AT calls YOUR webhook (POST /webhooks/simswap) with the result:
//     {"status":"Swapped"/"NotSwapped","lastSimSwapDate":"...","requestId":"..."}
//
// This means our /webhooks/simswap handler IS the integration — not a passive logger.
package at

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	// The SIM Swap API lives under the Insights subdomain — NOT api.africastalking.com.
	// Source: AfricasTalkingLtd/africastalking-python Insights.py SDK source.
	sandboxBaseURL    = "https://insights.sandbox.africastalking.com/v1"
	productionBaseURL = "https://insights.africastalking.com/v1"
	smsBaseURL        = "https://api.sandbox.africastalking.com" // SMS stays on the main API
	smsEndpoint       = "/version1/messaging"
	// invoke-check-simswap queues an async SIM swap check.
	// AT will POST the result to your registered callback URL.
	simSwapInvokeEndpoint = "/sim-swap/invoke-check-simswap"
)

// Client holds AT API credentials and an HTTP client with a sensible timeout.
type Client struct {
	APIKey      string
	Username    string
	Environment string // "sandbox" or "production"
	BaseURL     string
	HTTP        *http.Client
}

// InvokeCheckResponse is the immediate response from invoking a SIM swap check.
// The actual Swapped/NotSwapped result comes later via the status webhook.
type InvokeCheckResponse struct {
	Message  string `json:"message"`
	Response struct {
		Status        string `json:"status"`        // "Queued"
		RequestID     string `json:"requestId"`     // e.g. "ATSwpid_4032b7..."
		TransactionID string `json:"transactionId"` // UUID
	} `json:"response"`
}

// NewClientFromEnv builds a Client from environment variables.
// Required: AT_API_KEY, AT_USERNAME. Optional: AT_ENVIRONMENT (default: sandbox).
func NewClientFromEnv() (*Client, error) {
	apiKey := os.Getenv("AT_API_KEY")
	username := os.Getenv("AT_USERNAME")
	environment := os.Getenv("AT_ENVIRONMENT")

	if apiKey == "" || apiKey == "your_api_key_here" {
		return nil, fmt.Errorf("at: AT_API_KEY is not set — copy .env.example to .env and fill in your Africa's Talking credentials")
	}
	if username == "" {
		return nil, fmt.Errorf("at: AT_USERNAME is not set")
	}
	if environment == "" {
		environment = "sandbox"
	}

	base := sandboxBaseURL
	if environment == "production" {
		base = productionBaseURL
	}

	return &Client{
		APIKey:      apiKey,
		Username:    username,
		Environment: environment,
		BaseURL:     base,
		HTTP:        &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// InvokeSimSwapCheck asks AT to check whether phoneNumber (E.164) has been swapped.
// The result arrives asynchronously via the POST /webhooks/simswap callback.
// Returns the queued requestId on success so we can correlate the callback.
func (c *Client) InvokeSimSwapCheck(phoneNumber string) (*InvokeCheckResponse, error) {
	// AT expects the number WITHOUT the leading '+'.
	cleanPhone := strings.TrimPrefix(phoneNumber, "+")

	endpoint := fmt.Sprintf("%s%s?phone=%s", c.BaseURL, simSwapInvokeEndpoint, url.QueryEscape(cleanPhone))

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("at: build request: %w", err)
	}
	req.Header.Set("apiKey", c.APIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("at: InvokeSimSwapCheck HTTP: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("at: InvokeSimSwapCheck non-200 (%d): %s", resp.StatusCode, string(body))
	}

	var result InvokeCheckResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("at: decode InvokeCheckResponse: %w — raw: %s", err, string(body))
	}

	log.Printf("🔍 AT SIM swap check queued | phone=%s requestId=%s status=%s",
		phoneNumber, result.Response.RequestID, result.Response.Status)
	return &result, nil
}

// SendSMS sends a text message via the AT SMS API.
// to is an E.164 phone number. from is the sender ID (can be empty for sandbox).
func (c *Client) SendSMS(to, from, message string) error {
	// SMS lives on api.africastalking.com, not the Insights subdomain.
	smsDomain := "https://api.sandbox.africastalking.com"
	if c.Environment == "production" {
		smsDomain = "https://api.africastalking.com"
	}
	endpoint := smsDomain + smsEndpoint

	form := url.Values{}
	form.Set("username", c.Username)
	form.Set("to", to)
	form.Set("message", message)
	if from != "" {
		form.Set("from", from)
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("at: build SMS request: %w", err)
	}
	req.Header.Set("apiKey", c.APIKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("at: SendSMS HTTP: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("at: SendSMS non-200 (%d): %s", resp.StatusCode, string(body))
	}

	log.Printf("✅ SMS sent | to=%s status=%d", to, resp.StatusCode)
	return nil
}
