package webhooks

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/cabon-tech/not-my-sim/internal/events"
)

// resetPayload is the body for POST /events/reset.
type resetPayload struct {
	EventID     string    `json:"eventId"`     // idempotency key — supply any unique string
	PhoneNumber string    `json:"phoneNumber"` // E.164
	DeviceID    string    `json:"deviceId"`
	Location    string    `json:"location"`
	OccurredAt  time.Time `json:"occurredAt"`  // ISO 8601
}

// HandleReset handles POST /events/reset.
// Receives a PIN or password reset attempt event from the mobile money integration.
func (h *Handler) HandleReset(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var p resetPayload
	if err := json.Unmarshal(body, &p); err != nil {
		log.Printf("webhooks/reset: decode error: %v | raw: %s", err, string(body))
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if p.PhoneNumber == "" || p.EventID == "" {
		http.Error(w, "phoneNumber and eventId are required", http.StatusBadRequest)
		return
	}
	if p.OccurredAt.IsZero() {
		p.OccurredAt = time.Now().UTC()
	}

	log.Printf("🔑 Reset event received | phone=%s eventId=%s", p.PhoneNumber, p.EventID)

	// Idempotency: skip if already processed.
	_, err = h.DB.Exec(`
		INSERT INTO reset_events (phone_number, event_id, occurred_at, location)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (event_id) DO NOTHING`,
		p.PhoneNumber, p.EventID, p.OccurredAt, p.Location,
	)
	if err != nil {
		log.Printf("webhooks/reset: persist err: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Publish to event bus → state machine.
	h.Bus.ResetCh <- events.ResetEvent{
		EventID:     p.EventID,
		PhoneNumber: p.PhoneNumber,
		DeviceID:    p.DeviceID,
		Location:    p.Location,
		OccurredAt:  p.OccurredAt,
		ReceivedAt:  time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"received": "ok", "eventId": p.EventID})
}
