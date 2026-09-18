// Package webhooks contains HTTP handlers for incoming event callbacks.
// The Handler struct holds shared dependencies (DB + event bus) injected from main.
package webhooks

import (
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/cabon-tech/not-my-sim/internal/events"
)

// Handler holds shared dependencies for all webhook/event handlers.
type Handler struct {
	DB  *sql.DB
	Bus *events.Bus
}

// NewHandler constructs a Handler with the given dependencies.
func NewHandler(db *sql.DB, bus *events.Bus) *Handler {
	return &Handler{DB: db, Bus: bus}
}

// --- POST /webhooks/simswap ---

// simSwapPayload is what Africa's Talking POSTs to our callback URL.
// PhoneNumber is NOT in the real AT payload — it's here for demo curl testing.
// In production, we look it up from pending_checks using RequestID.
type simSwapPayload struct {
	Status          string `json:"status"`          // "Swapped", "NotSwapped", "Failed"
	LastSimSwapDate string `json:"lastSimSwapDate"` // "DD-MM-YYYY"
	ProviderRefID   string `json:"providerRefId"`
	RequestID       string `json:"requestId"`
	TransactionID   string `json:"transactionId"`
	// Optional — used in demo curl calls so we don't need pending_checks lookup.
	PhoneNumber string `json:"phoneNumber"`
	AgentID     string `json:"agentId"`
}

// HandleSimSwap handles POST /webhooks/simswap — AT's async SIM swap status callback.
func (h *Handler) HandleSimSwap(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var p simSwapPayload
	if err := json.Unmarshal(body, &p); err != nil {
		log.Printf("webhooks/simswap: decode error: %v | raw: %s", err, string(body))
		w.WriteHeader(http.StatusOK) // still 200 — don't let AT retry bad payloads
		return
	}

	log.Printf("📲 SIM swap callback | requestId=%s status=%s swapDate=%s",
		p.RequestID, p.Status, p.LastSimSwapDate)

	// Resolve phone number: explicit field (demo) OR pending_checks lookup (production).
	phone := p.PhoneNumber
	if phone == "" && p.RequestID != "" {
		phone = h.lookupPhoneByRequestID(p.RequestID)
	}
	if phone == "" {
		log.Printf("webhooks/simswap: cannot resolve phone number for requestId=%s — dropping event", p.RequestID)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Idempotency: skip if already processed.
	if !h.persistSwapEvent(phone, p.RequestID, p.Status, p.LastSimSwapDate, p.AgentID) {
		log.Printf("webhooks/simswap: duplicate requestId=%s — skipped", p.RequestID)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
		return
	}

	// Publish to the internal state machine.
	h.Bus.SwapCh <- events.SwapEvent{
		PhoneNumber: phone,
		Status:      p.Status,
		RequestID:   p.RequestID,
		SwapDateStr: p.LastSimSwapDate,
		AgentID:     p.AgentID,
		ReceivedAt:  time.Now().UTC(),
	}

	if p.Status == "Swapped" {
		log.Printf("🚨 SWAP CONFIRMED | phone=%s requestId=%s agent=%s — published to state machine", phone, p.RequestID, p.AgentID)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (h *Handler) lookupPhoneByRequestID(requestID string) string {
	var phone string
	err := h.DB.QueryRow(
		`SELECT phone_number FROM pending_checks WHERE request_id = $1`, requestID,
	).Scan(&phone)
	if err == sql.ErrNoRows {
		return ""
	}
	if err != nil {
		log.Printf("webhooks/simswap: pending_checks lookup err: %v", err)
		return ""
	}
	return phone
}

// persistSwapEvent inserts the swap event. Returns false if already exists (duplicate).
func (h *Handler) persistSwapEvent(phone, requestID, status, swapDateStr, agentID string) bool {
	if agentID == "" {
		agentID = "system"
	}
	_, err := h.DB.Exec(`
		INSERT INTO swap_events (phone_number, request_id, status, swap_date_str, agent_id)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (request_id) DO NOTHING`,
		phone, requestID, status, swapDateStr, agentID,
	)
	if err != nil {
		log.Printf("webhooks/simswap: persist err: %v", err)
		return true // on error, still publish — better to re-process than drop
	}
	// Check if it was actually inserted (not a conflict).
	var count int
	h.DB.QueryRow(`SELECT COUNT(*) FROM swap_events WHERE request_id = $1`, requestID).Scan(&count)
	return count == 1
}
