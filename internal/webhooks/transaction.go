package webhooks

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/cabon-tech/not-my-sim/internal/events"
)

// transactionPayload is the body for POST /events/transaction.
type transactionPayload struct {
	TransactionID string    `json:"transactionId"`
	FromNumber    string    `json:"fromNumber"`
	ToNumber      string    `json:"toNumber"`
	Amount        float64   `json:"amount"`
	DeviceID      string    `json:"deviceId"`
	Location      string    `json:"location"`
	OccurredAt    time.Time `json:"occurredAt"`
}

// HandleTransaction handles POST /events/transaction.
// Receives a money transfer event from the mobile money / banking integration.
func (h *Handler) HandleTransaction(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var p transactionPayload
	if err := json.Unmarshal(body, &p); err != nil {
		log.Printf("webhooks/transaction: decode error: %v | raw: %s", err, string(body))
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if p.TransactionID == "" || p.FromNumber == "" || p.ToNumber == "" {
		http.Error(w, "transactionId, fromNumber, and toNumber are required", http.StatusBadRequest)
		return
	}
	if p.OccurredAt.IsZero() {
		p.OccurredAt = time.Now().UTC()
	}

	log.Printf("💸 Transfer event received | txId=%s from=%s to=%s amount=%.2f KES",
		p.TransactionID, p.FromNumber, p.ToNumber, p.Amount)

	// Idempotency: skip if already processed.
	_, err = h.DB.Exec(`
		INSERT INTO transfer_events (transaction_id, from_number, to_number, amount, occurred_at, location)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (transaction_id) DO NOTHING`,
		p.TransactionID, p.FromNumber, p.ToNumber, p.Amount, p.OccurredAt, p.Location,
	)
	if err != nil {
		log.Printf("webhooks/transaction: persist err: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Publish to event bus → state machine.
	h.Bus.TransferCh <- events.TransferEvent{
		TransactionID: p.TransactionID,
		FromNumber:    p.FromNumber,
		ToNumber:      p.ToNumber,
		Amount:        p.Amount,
		DeviceID:      p.DeviceID,
		Location:      p.Location,
		OccurredAt:    p.OccurredAt,
		ReceivedAt:    time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"received": "ok", "transactionId": p.TransactionID})
}
