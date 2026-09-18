// Package events defines the in-process event types and the channel bus
// that replaces Kafka in the hackathon build.
//
// Production equivalent: Kafka topics swap_events, txn_events, reset_events.
// Here each topic is a buffered Go channel, partitioned by phone number
// inside the state machine (single goroutine, keyed dispatch).
package events

import "time"

// SwapEvent is published when AT's Insights API reports a SIM swap status
// via the POST /webhooks/simswap callback.
type SwapEvent struct {
	PhoneNumber  string    // resolved from pending_checks or direct payload field
	RequestID    string    // AT's requestId — used for idempotency
	Status       string    // "Swapped", "NotSwapped", "Failed", "Pending"
	SwapDateStr  string    // "DD-MM-YYYY" string from AT
	AgentID      string
	ReceivedAt   time.Time
}

// SimSwapEvent represents a confirmed SIM swap from the telecom.
type SimSwapEvent struct {
	PhoneNumber string    `json:"phoneNumber"`
	Status      string    `json:"status"`
	RequestID   string    `json:"requestId"`
	AgentID     string    `json:"agentId"`
	OccurredAt  time.Time `json:"occurredAt"`
	ReceivedAt  time.Time `json:"-"`
}

// ResetEvent represents a PIN or password reset attempt.
type ResetEvent struct {
	EventID     string    `json:"eventId"`
	PhoneNumber string    `json:"phoneNumber"`
	DeviceID    string    `json:"deviceId"`
	Location    string    `json:"location"`
	OccurredAt  time.Time `json:"occurredAt"`
	ReceivedAt  time.Time `json:"-"`
}

// TransferEvent represents a mobile money transfer.
type TransferEvent struct {
	TransactionID string    `json:"transactionId"`
	FromNumber    string    `json:"fromNumber"`
	ToNumber      string    `json:"toNumber"`
	Amount        float64   `json:"amount"`
	DeviceID      string    `json:"deviceId"`
	Location      string    `json:"location"`
	OccurredAt    time.Time `json:"occurredAt"`
	ReceivedAt    time.Time `json:"-"`
}

// Bus holds in-process buffered channels for all event types.
// Buffer size 256 is enough for demo burst rates without blocking the HTTP handler.
type Bus struct {
	SwapCh     chan SwapEvent
	ResetCh    chan ResetEvent
	TransferCh chan TransferEvent
}

// New creates a Bus with buffered channels ready to use.
func New() *Bus {
	return &Bus{
		SwapCh:     make(chan SwapEvent, 256),
		ResetCh:    make(chan ResetEvent, 256),
		TransferCh: make(chan TransferEvent, 256),
	}
}
