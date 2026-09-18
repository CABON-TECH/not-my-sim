package webhooks

import (
	"log"
	"net/http"
	"strings"
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
		// User blocked the transaction
		log.Printf("🛑 2FA BLOCK INITIATED | User %s denied the transaction. Freezing account.", from)
		h.DB.Exec(`UPDATE account_state SET is_frozen = TRUE WHERE phone_number = $1`, from)
	} else if text == "1" {
		log.Printf("✅ 2FA APPROVED | User %s approved the transaction.", from)
	} else {
		log.Printf("⚠️ 2FA UNKNOWN REPLY | User %s sent: %s", from, text)
	}

	w.WriteHeader(http.StatusOK)
}
