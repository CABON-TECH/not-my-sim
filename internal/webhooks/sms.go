package webhooks

import (
	"log"
	"net/http"
	"strings"

	"github.com/cabon-tech/not-my-sim/internal/actions"
)

// HandleSMSCallback receives the interactive SMS replies from Africa's Talking.
func (h *Handler) HandleSMSCallback(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	from := r.FormValue("from")
	// Fix URL decoding: '+' is decoded as ' ', so we must restore it for E.164 format.
	if len(from) > 0 && from[0] == ' ' {
		from = "+" + from[1:]
	}
	text := strings.TrimSpace(r.FormValue("text"))

	if from == "" || text == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	log.Printf("📱 2FA SMS REPLY | from=%s text='%s'", from, text)

	if text == "2" {
		// User blocked the transaction — freeze account and confirm via SMS.
		log.Printf("🛑 2FA BLOCK INITIATED | User %s denied the transaction. Freezing account.", from)
		h.DB.Exec(`UPDATE account_state SET is_frozen = TRUE WHERE phone_number = $1`, from)
		actions.Send2FASMS(from,
			"NOT-MY-SIM: Your account has been FROZEN and the suspicious transaction BLOCKED. "+
				"Your funds are safe. Contact your mobile money provider to restore access.")
	} else if text == "1" {
		log.Printf("✅ 2FA APPROVED | User %s approved the transaction.", from)
		actions.Send2FASMS(from,
			"NOT-MY-SIM: Transaction APPROVED. If you did not authorise this, "+
				"reply 2 immediately or call your provider.")
	} else {
		log.Printf("⚠️ 2FA UNKNOWN REPLY | User %s sent: %s", from, text)
		actions.Send2FASMS(from,
			"NOT-MY-SIM: Invalid response. Reply 1 to APPROVE or 2 to BLOCK the transaction.")
	}

	w.WriteHeader(http.StatusOK)
}
