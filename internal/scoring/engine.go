// Package scoring implements the risk scoring engine described in design doc §3.6.
//
// It combines three signal families into a single score + reason codes:
//   - CEP state (deterministic, rule-based — always available, zero latency)
//   - Graph network score (hub-scoring query — is the recipient a known mule hub?)
//   - ML model score (out of scope for hackathon — documented as next iteration)
//
// The output is a score (0–100) + human-readable reason codes, not just a number.
// Reason codes are critical for both judge legibility and real-world compliance/audit.
package scoring

import (
	"database/sql"
	"log"
	"time"

	"github.com/cabon-tech/not-my-sim/internal/graph"
)

// Action is the recommended response to a given risk score.
type Action string

const (
	ActionLogOnly          Action = "LOG_ONLY"
	ActionSMSAlert         Action = "SMS_ALERT"
	ActionSMSAndStepUp     Action = "SMS_ALERT_STEP_UP_AUTH"
	ActionSMSAndHold       Action = "SMS_ALERT_TRANSACTION_HOLD"
)

// Result is the output of the scoring engine for one evaluation.
type Result struct {
	PhoneNumber string    `json:"phoneNumber"`
	Score       int       `json:"score"`       // 0–100 combined risk score
	CEPRisk     int       `json:"cepRisk"`     // contribution from state machine state
	GraphRisk   int       `json:"graphRisk"`   // contribution from hub-score
	ReasonCodes []string  `json:"reasonCodes"` // human-readable, auditable
	Action      Action    `json:"action"`      // recommended response
	ComputedAt  time.Time `json:"computedAt"`
}

// Thresholds — documented here, not magic numbers.
// These match the table in the implementation plan.
const (
	thresholdSMSAlert  = 40 // score ≥ 40 → send SMS alert to victim
	thresholdStepUp    = 70 // score ≥ 70 → require step-up auth for pending txn
	thresholdHold      = 85 // score ≥ 85 → hold transaction + alert
)

// CEP state → base risk level. Must match statemachine.State.RiskLevel().
// We accept the state as a plain string to keep scoring independent of statemachine.
var stateRisk = map[string]int{
	"NORMAL":                       0,
	"SWAP_DETECTED":                30,
	"CREDENTIAL_RESET_POST_SWAP":   50,
	"TRANSFER_POST_RESET":          70,
}

// Hub risk bonus by RiskLabel (from graph.HubResult.RiskLabel).
// Adds on top of the CEP risk to push borderline cases over thresholds.
var hubRiskBonus = map[string]int{
	"SYNDICATE_CANDIDATE":      5,  // 2 victims   → e.g. 90+5 = 95 → HOLD
	"PROBABLE_SYNDICATE_HUB":   10, // 3-4 victims → e.g. 40+10 = 50 → SMS_ALERT
	"CONFIRMED_SYNDICATE_HUB":  15, // 5+ victims  → e.g. 70+15 = 85 → HOLD
}

// Evaluate computes a risk score for fromPhone given its current CEP state
// and (optionally) the recipient phone number, deviceID, and transferLoc for graph/hub enrichment.
//
// stateStr is the string value of statemachine.State (e.g. "SWAP_DETECTED").
func Evaluate(db *sql.DB, fromPhone, stateStr, toPhone, deviceID, transferLoc string) Result {
	reasons := []string{}
	computedAt := time.Now().UTC()

	// 1. CEP state risk — deterministic, always available.
	cepRisk := stateRisk[stateStr]
	if stateStr != "NORMAL" && stateStr != "" {
		reasons = append(reasons, "CEP_STATE_"+stateStr)
	}

	// 2. Graph network risk — hub-score enrichment on the recipient.
	graphRisk := 0
	if toPhone != "" && db != nil {
		if hubs, err := graph.QueryHubScore(db, 2, 24); err == nil {
			for _, h := range hubs {
				if h.Recipient == toPhone {
					graphRisk += hubRiskBonus[h.RiskLabel]
					reasons = append(reasons,
						"HUB_RECIPIENT_"+h.RiskLabel,
						"HUB_VICTIM_COUNT_"+itoa(h.VictimCount),
					)
					break
				}
			}
		} else {
			log.Printf("scoring: hub-score query err: %v", err)
		}
	}

	// 3. Device Collusion risk — +30 if device used across multiple accounts.
	deviceRisk := 0
	if deviceID != "" && db != nil {
		if victimCount, err := graph.QueryDeviceCollusion(db, deviceID); err == nil {
			if victimCount >= 2 {
				deviceRisk = 30
				reasons = append(reasons, "DEVICE_COLLUSION_DETECTED")
			}
		} else {
			log.Printf("scoring: device collusion query err: %v", err)
		}
	}

	// 4. Geo-Velocity / Impossible Travel Risk
	travelRisk := 0
	if transferLoc != "" && db != nil {
		var resetLoc string
		err := db.QueryRow(`
			SELECT location FROM reset_events 
			WHERE phone_number = $1 
			ORDER BY occurred_at DESC LIMIT 1`, fromPhone).Scan(&resetLoc)
		if err == nil && resetLoc != "" && transferLoc != resetLoc {
			travelRisk = 20
			reasons = append(reasons, "IMPOSSIBLE_TRAVEL_DETECTED")
			log.Printf("🌍 GEO-VELOCITY FLAG | reset=%s transfer=%s", resetLoc, transferLoc)
		}
	}

	// 5. Combined score, capped at 100.
	score := cepRisk + graphRisk + deviceRisk + travelRisk
	if score > 100 {
		score = 100
	}

	// 6. Recommended action based on thresholds.
	action := scoreToAction(score)

	return Result{
		PhoneNumber: fromPhone,
		Score:       score,
		CEPRisk:     cepRisk,
		GraphRisk:   graphRisk + deviceRisk + travelRisk,
		ReasonCodes: reasons,
		Action:      action,
		ComputedAt:  computedAt,
	}
}

func scoreToAction(score int) Action {
	switch {
	case score >= thresholdHold:
		return ActionSMSAndHold
	case score >= thresholdStepUp:
		return ActionSMSAndStepUp
	case score >= thresholdSMSAlert:
		return ActionSMSAlert
	default:
		return ActionLogOnly
	}
}

func itoa(n int) string {
	if n < 0 {
		return "0"
	}
	digits := []byte("0123456789")
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 3)
	for n > 0 {
		buf = append([]byte{digits[n%10]}, buf...)
		n /= 10
	}
	return string(buf)
}
