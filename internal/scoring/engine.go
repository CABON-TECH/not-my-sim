
package scoring

import (
	"database/sql"
	"log"
	"time"

	"github.com/cabon-tech/not-my-sim/internal/graph"
)


type Action string

const (
	ActionLogOnly          Action = "LOG_ONLY"
	ActionSMSAlert         Action = "SMS_ALERT"
	ActionSMSAndStepUp     Action = "SMS_ALERT_STEP_UP_AUTH"
	ActionSMSAndHold       Action = "SMS_ALERT_TRANSACTION_HOLD"
)


type Result struct {
	PhoneNumber string    `json:"phoneNumber"`
	Score       int       `json:"score"`       
	CEPRisk     int       `json:"cepRisk"`     
	GraphRisk   int       `json:"graphRisk"`   
	ReasonCodes []string  `json:"reasonCodes"` 
	Action      Action    `json:"action"`      
	ComputedAt  time.Time `json:"computedAt"`
}


const (
	thresholdSMSAlert  = 40 
	thresholdStepUp    = 70 
	thresholdHold      = 85 
)


var stateRisk = map[string]int{
	"NORMAL":                       0,
	"SWAP_DETECTED":                30,
	"CREDENTIAL_RESET_POST_SWAP":   50,
	"TRANSFER_POST_RESET":          70,
}


var hubRiskBonus = map[string]int{
	"SYNDICATE_CANDIDATE":      5,  
	"PROBABLE_SYNDICATE_HUB":   10, 
	"CONFIRMED_SYNDICATE_HUB":  15, 
}


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
