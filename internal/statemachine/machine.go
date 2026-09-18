// Package statemachine implements the per-account CEP (Complex Event Processing)
// state machine described in the design doc Section 3.3.
//
// States:
//
//	NORMAL → (swap event) → SWAP_DETECTED
//	SWAP_DETECTED → (reset within window) → CREDENTIAL_RESET_POST_SWAP
//	CREDENTIAL_RESET_POST_SWAP → (transfer within window) → TRANSFER_POST_RESET
//	Any non-NORMAL state → (window expires with no further events) → NORMAL
//
// Production equivalent: Apache Flink CEP job with per-key state, running on
// the Kafka event stream. Here: a single goroutine consuming all three in-process
// channels, maintaining an in-memory state map backed by Postgres.
package statemachine

import (
	"database/sql"
	"log"
	"sync"
	"time"

	"github.com/cabon-tech/not-my-sim/internal/events"
	"github.com/cabon-tech/not-my-sim/internal/graph"
)

// State represents the fraud-risk state of a single account.
type State string

const (
	StateNormal                  State = "NORMAL"
	StateSwapDetected            State = "SWAP_DETECTED"
	StateCredentialResetPostSwap State = "CREDENTIAL_RESET_POST_SWAP"
	StateTransferPostReset       State = "TRANSFER_POST_RESET"
)

// RiskLevel returns a 0–100 numeric risk for a state.
// Used by the scoring engine in Phase 4.
func (s State) RiskLevel() int {
	switch s {
	case StateSwapDetected:
		return 40
	case StateCredentialResetPostSwap:
		return 70
	case StateTransferPostReset:
		return 90
	default:
		return 0
	}
}

// AccountState is the in-memory + persisted state for one account.
type AccountState struct {
	PhoneNumber         string
	State               State
	StateChangedAt      time.Time
	RiskWindowExpiresAt *time.Time // nil when NORMAL
	LastSwapRequestID   string
}

// OnCriticalFn is called (in a goroutine) when an account reaches TRANSFER_POST_RESET.
// Phase 4 will wire in SMS alerts and the transaction hold.
// Signature matches what the// OnCriticalFn is a callback fired when the state machine reaches TRANSFER_POST_RESET.
type OnCriticalFn func(fromNumber, toNumber string, amount float64, deviceId, transferLoc string)

// Machine is the state machine worker. Create with New, then call Start.
type Machine struct {
	db            *sql.DB
	bus           *events.Bus
	states        map[string]*AccountState // in-memory cache, phone → state
	mu            sync.Mutex
	riskWindowDur time.Duration
	onCritical    OnCriticalFn
}

// New creates a Machine. riskWindowHours is how long after a swap we consider
// subsequent events high-risk (72h in the design doc).
func New(db *sql.DB, bus *events.Bus, riskWindowHours int, onCritical OnCriticalFn) *Machine {
	return &Machine{
		db:            db,
		bus:           bus,
		states:        make(map[string]*AccountState),
		riskWindowDur: time.Duration(riskWindowHours) * time.Hour,
		onCritical:    onCritical,
	}
}

// Start launches the state machine worker goroutine.
// Call once from main after the DB is ready.
func (m *Machine) Start() {
	log.Printf("🔄 State machine started | risk window = %v", m.riskWindowDur)
	expireTicker := time.NewTicker(1 * time.Minute)

	go func() {
		for {
			select {
			case ev := <-m.bus.SwapCh:
				m.handleSwap(ev)
			case ev := <-m.bus.ResetCh:
				m.handleReset(ev)
			case ev := <-m.bus.TransferCh:
				m.handleTransfer(ev)
			case <-expireTicker.C:
				m.checkWindowExpiry()
			}
		}
	}()
}

// GetState returns the current state for a phone number (for the risk scoring engine).
func (m *Machine) GetState(phoneNumber string) State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.loadOrInit(phoneNumber).State
}

// --- event handlers ---

func (m *Machine) handleSwap(ev events.SwapEvent) {
	if ev.Status != "Swapped" {
		log.Printf("state: phone=%s status=%s — no transition (not confirmed swap)", ev.PhoneNumber, ev.Status)
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	st := m.loadOrInit(ev.PhoneNumber)
	if st.State != StateNormal {
		log.Printf("state: phone=%s already in %s — ignoring swap (may be duplicate)", ev.PhoneNumber, st.State)
		return
	}

	expiry := time.Now().UTC().Add(m.riskWindowDur)
	st.State = StateSwapDetected
	st.StateChangedAt = time.Now().UTC()
	st.RiskWindowExpiresAt = &expiry
	st.LastSwapRequestID = ev.RequestID
	m.persist(st)

	log.Printf("🔁 TRANSITION | phone=%s  NORMAL → SWAP_DETECTED | risk window until %s",
		ev.PhoneNumber, expiry.Format(time.RFC3339))
}

func (m *Machine) handleReset(ev events.ResetEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	st := m.loadOrInit(ev.PhoneNumber)

	switch st.State {
	case StateNormal:
		log.Printf("state: phone=%s NORMAL — reset is benign (no prior swap)", ev.PhoneNumber)
		return
	case StateSwapDetected:
		if m.windowExpired(st) {
			log.Printf("state: phone=%s window expired — reset treated as benign, resetting to NORMAL", ev.PhoneNumber)
			m.transitionToNormal(st)
			return
		}
		st.State = StateCredentialResetPostSwap
		st.StateChangedAt = time.Now().UTC()
		m.persist(st)
		log.Printf("🔁 TRANSITION | phone=%s  SWAP_DETECTED → CREDENTIAL_RESET_POST_SWAP  ⚠️  HIGH RISK",
			ev.PhoneNumber)
	default:
		log.Printf("state: phone=%s in %s — reset received (already past reset stage)", ev.PhoneNumber, st.State)
	}

	// Write device edge if a device ID was provided
	if ev.DeviceID != "" && m.db != nil {
		graph.WriteDeviceEdge(m.db, ev.PhoneNumber, ev.DeviceID)
	}
}

func (m *Machine) handleTransfer(ev events.TransferEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	st := m.loadOrInit(ev.FromNumber)

	switch st.State {
	case StateNormal, StateSwapDetected:
		log.Printf("state: phone=%s in %s — transfer is not post-reset (low/medium risk)", ev.FromNumber, st.State)
		return
	case StateCredentialResetPostSwap:
		if m.windowExpired(st) {
			log.Printf("state: phone=%s window expired — transfer treated as benign", ev.FromNumber)
			m.transitionToNormal(st)
			return
		}
		st.State = StateTransferPostReset
		st.StateChangedAt = time.Now().UTC()
		m.persist(st)
		log.Printf("🚨 CRITICAL | phone=%s  CREDENTIAL_RESET_POST_SWAP → TRANSFER_POST_RESET | to=%s amount=%.2f",
			ev.FromNumber, ev.ToNumber, ev.Amount)

		// Write device edge if a device ID was provided
		if ev.DeviceID != "" && m.db != nil {
			graph.WriteDeviceEdge(m.db, ev.FromNumber, ev.DeviceID)
		}

		// Trigger action layer immediately
		if m.onCritical != nil {
			m.onCritical(ev.FromNumber, ev.ToNumber, ev.Amount, ev.DeviceID, ev.Location)
		}
	case StateTransferPostReset:
		log.Printf("🚨 REPEAT TRANSFER in TRANSFER_POST_RESET | phone=%s to=%s amount=%.2f — already flagged",
			ev.FromNumber, ev.ToNumber, ev.Amount)
		if m.onCritical != nil {
			m.onCritical(ev.FromNumber, ev.ToNumber, ev.Amount, ev.DeviceID, ev.Location)
		}
	}
}

// checkWindowExpiry runs on a 1-minute ticker, resetting any accounts whose
// risk window has expired without completing the fraud chain.
func (m *Machine) checkWindowExpiry() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, st := range m.states {
		if st.State != StateNormal && m.windowExpired(st) {
			log.Printf("⏱️  window expired | phone=%s %s → NORMAL", st.PhoneNumber, st.State)
			m.transitionToNormal(st)
		}
	}
}

// --- helpers ---

func (m *Machine) windowExpired(st *AccountState) bool {
	if st.RiskWindowExpiresAt == nil {
		return false
	}
	return time.Now().UTC().After(*st.RiskWindowExpiresAt)
}

func (m *Machine) transitionToNormal(st *AccountState) {
	st.State = StateNormal
	st.StateChangedAt = time.Now().UTC()
	st.RiskWindowExpiresAt = nil
	m.persist(st)
}

// loadOrInit returns the AccountState from the in-memory cache,
// or loads it from Postgres, or creates a fresh NORMAL state.
// Caller must hold m.mu.
func (m *Machine) loadOrInit(phoneNumber string) *AccountState {
	if st, ok := m.states[phoneNumber]; ok {
		return st
	}
	st := m.loadFromDB(phoneNumber)
	if st == nil {
		st = &AccountState{
			PhoneNumber:    phoneNumber,
			State:          StateNormal,
			StateChangedAt: time.Now().UTC(),
		}
	}
	m.states[phoneNumber] = st
	return st
}

func (m *Machine) loadFromDB(phoneNumber string) *AccountState {
	row := m.db.QueryRow(`
		SELECT phone_number, state, state_changed_at, risk_window_expires_at, last_swap_request_id
		FROM account_state WHERE phone_number = $1`, phoneNumber)

	st := &AccountState{}
	var expiresAt sql.NullTime
	var reqID sql.NullString

	err := row.Scan(&st.PhoneNumber, &st.State, &st.StateChangedAt, &expiresAt, &reqID)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		log.Printf("state: loadFromDB phone=%s err=%v", phoneNumber, err)
		return nil
	}
	if expiresAt.Valid {
		t := expiresAt.Time
		st.RiskWindowExpiresAt = &t
	}
	if reqID.Valid {
		st.LastSwapRequestID = reqID.String
	}
	return st
}

// persist upserts the account state to Postgres.
// Must be called while holding m.mu (no additional lock taken here).
func (m *Machine) persist(st *AccountState) {
	_, err := m.db.Exec(`
		INSERT INTO account_state
		    (phone_number, state, state_changed_at, risk_window_expires_at, last_swap_request_id)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (phone_number) DO UPDATE SET
		    state                  = EXCLUDED.state,
		    state_changed_at       = EXCLUDED.state_changed_at,
		    risk_window_expires_at = EXCLUDED.risk_window_expires_at,
		    last_swap_request_id   = EXCLUDED.last_swap_request_id`,
		st.PhoneNumber,
		string(st.State),
		st.StateChangedAt,
		st.RiskWindowExpiresAt,
		nullableString(st.LastSwapRequestID),
	)
	if err != nil {
		log.Printf("state: persist phone=%s err=%v", st.PhoneNumber, err)
	}
}

func nullableString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
