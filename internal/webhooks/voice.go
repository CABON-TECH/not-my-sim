package webhooks

import (
	"fmt"
	"log"
	"net/http"
)

// HandleVoiceCallback responds to Africa's Talking when the user picks up the phone.
// AT requires an XML response telling it what to do (e.g., <Say> something).
func HandleVoiceCallback(w http.ResponseWriter, r *http.Request) {
	// Log the incoming call details from AT
	r.ParseForm()
	callSessionId := r.FormValue("sessionId")
	isActive := r.FormValue("isActive") // "1" if active, "0" if completed
	callerNumber := r.FormValue("callerNumber")

	if isActive == "1" {
		log.Printf("🎙️  VOICE CALL ANSWERED | session=%s to=%s", callSessionId, callerNumber)
		
		xmlResponse := `<?xml version="1.0" encoding="UTF-8"?>
<Response>
	<Say voice="man" playBeep="false">Red Alert. This is the Not-My-Sim Fraud Engine. We have detected a critical SIM Swap and illegal transfer on your account. Your funds have been frozen automatically for your protection. Please visit the nearest branch with your physical ID.</Say>
</Response>`

		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, xmlResponse)
	} else {
		log.Printf("🛑 VOICE CALL ENDED | session=%s", callSessionId)
		w.WriteHeader(http.StatusOK)
	}
}
