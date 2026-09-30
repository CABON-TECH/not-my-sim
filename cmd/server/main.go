package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"

	"github.com/cabon-tech/not-my-sim/internal/actions"
	"github.com/cabon-tech/not-my-sim/internal/at"
	"github.com/cabon-tech/not-my-sim/internal/events"
	"github.com/cabon-tech/not-my-sim/internal/graph"
	"github.com/cabon-tech/not-my-sim/internal/platform"
	"github.com/cabon-tech/not-my-sim/internal/scoring"
	"github.com/cabon-tech/not-my-sim/internal/statemachine"
	"github.com/cabon-tech/not-my-sim/internal/ui"
	"github.com/cabon-tech/not-my-sim/internal/webhooks"
)

func main() {
	// Load .env if present.
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  No .env file — using environment variables directly")
	}

	// 1. Connect to Postgres.
	if err := platform.InitDB(); err != nil {
		log.Fatalf("❌ DB init failed: %v", err)
	}
	defer platform.DB.Close()

	// 2. Validate AT credentials.
	atClient, err := at.NewClientFromEnv()
	if err != nil {
		log.Fatalf("❌ AT client init failed: %v", err)
	}

	// 3. AT gate check — invoke an async SIM swap check for one number.
	//    The result arrives via POST /webhooks/simswap a few seconds later.
	//    Replace with your real sandbox number; leave as-is if AT Insights isn't enabled yet.
	testPhone := os.Getenv("GATE_CHECK_PHONE")
	if testPhone == "" {
		testPhone = "+254700000001"
	}
	log.Printf("🔍 Gate check: invoking AT SIM Swap check for %s ...", testPhone)
	if invoke, err := atClient.InvokeSimSwapCheck(testPhone); err != nil {
		log.Printf("⚠️  AT gate check failed (non-fatal): %v", err)
	} else {
		log.Printf("✅ AT gate check passed | requestId=%s status=%s", invoke.Response.RequestID, invoke.Response.Status)
		// Store pending check so the webhook can resolve phone → requestId.
		platform.DB.Exec(
			`INSERT INTO pending_checks (request_id, phone_number) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			invoke.Response.RequestID, testPhone,
		)
	}

	// 4. Build the event bus (in-process channels, Kafka substitute).
	bus := events.New()
	log.Println("✅ Event bus ready")

	// 5. Build and start the CEP state machine.
	riskWindowHours := 72
	if v := os.Getenv("RISK_WINDOW_HOURS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			riskWindowHours = n
		}
	}

	// onCritical fires when a TRANSFER_POST_RESET transition completes.
	// Phase 4: Evaluate risk, write graph edge, and trigger SMS/Hold actions.
	onCritical := func(fromNumber, toNumber string, amount float64, deviceId, transferLoc string) {
		log.Printf("🚨🚨 CRITICAL FRAUD ALERT | victim=%s recipient=%s amount=%.2f KES",
			fromNumber, toNumber, amount)

		// Write graph edge: victim → recipient (Phase 3 — graph ingestion).
		if err := graph.WriteEdge(platform.DB, fromNumber, toNumber); err != nil {
			log.Printf("   ⚠️  graph edge write failed: %v", err)
		}

		// Phase 4: Evaluate risk score.
		// Note: The machine is already in TRANSFER_POST_RESET.
		state := "TRANSFER_POST_RESET"
		result := scoring.Evaluate(platform.DB, fromNumber, state, toNumber, deviceId, transferLoc)
		
		log.Printf("   → Risk Score: %d/100 | Action: %s | Reasons: %v", 
			result.Score, result.Action, result.ReasonCodes)

		// Freeze the account in the DB if score crosses threshold
		if result.Score >= 85 {
			log.Printf("🛑 FREEZE ACCOUNT | phone=%s score=%d", fromNumber, result.Score)
			_, err := platform.DB.Exec(`UPDATE account_state SET is_frozen = TRUE WHERE phone_number = $1`, fromNumber)
			if err != nil {
				log.Printf("   ⚠️ failed to freeze account in db: %v", err)
			} else {
				log.Printf("   ❄️ ACCOUNT FROZEN | phone=%s", fromNumber)
			}

			// If it's a 100/100 guaranteed fraud, trigger the extreme Robocall
			if result.Score == 100 {
				actions.InitiateRobocall(fromNumber)
			}
		} else if result.Score >= 65 {
			log.Printf("⚠️ SUSPICIOUS ACTIVITY | phone=%s score=%d | Triggering Interactive 2FA", fromNumber, result.Score)
			actions.Send2FASMS(fromNumber, "Not-My-SIM Alert: Suspicious transfer of KES detected. Reply 1 to APPROVE, or 2 to BLOCK and freeze account.")
		}

		if result.Action == "SMS_ALERT_TRANSACTION_HOLD" {
			go func() {
				// 1. Send SMS
				actions.SendFraudAlert(atClient, fromNumber, toNumber, amount, result.Score)

				// 2. Automated Voice Call for max risk
				if result.Score == 100 {
					actions.InitiateRobocall(fromNumber)
				}
			}()
		}
	}

	machine := statemachine.New(platform.DB, bus, riskWindowHours, onCritical)
	machine.Start()

	// 6. Build webhook handler (Phase 2 — full ingestion pipeline).
	h := webhooks.NewHandler(platform.DB, bus)

	// 7. Build router.
	r := mux.NewRouter()
	r.Use(loggingMiddleware)

	// Health check.
	r.HandleFunc("/health", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"not-my-sim"}`))
	}).Methods(http.MethodGet)

	// Phase 1: SIM swap webhook (now with persistence + event bus).
	r.HandleFunc("/webhooks/simswap", h.HandleSimSwap).Methods(http.MethodPost)
	r.HandleFunc("/webhooks/sms", h.HandleSMSCallback).Methods(http.MethodPost)
	r.HandleFunc("/webhooks/voice", webhooks.HandleVoiceCallback).Methods(http.MethodPost)

	// Phase 2: event ingestion endpoints.
	r.HandleFunc("/events/transaction", h.HandleTransaction).Methods(http.MethodPost)
	r.HandleFunc("/events/reset", h.HandleReset).Methods(http.MethodPost)

	// Phase 3: graph hub-scoring query.
	// GET /graph/hub-score?min_victims=2&window_hours=24
	r.HandleFunc("/graph/hub-score", graph.HandleHubScore(platform.DB)).Methods(http.MethodGet)

	// Demo admin: reset event tables + account states to clean seed state.
	// NOT for production — demo convenience only.
	r.HandleFunc("/admin/reset-demo", handleResetDemo).Methods(http.MethodPost)

	// Phase 4: simulated transaction hold.
	r.HandleFunc("/simulate/hold", actions.HandleSimulatedHold(platform.DB, machine)).Methods(http.MethodPost)

	// UI Dashboard Routes (HTMX)
	r.HandleFunc("/", ui.HandleIndex).Methods(http.MethodGet)
	r.HandleFunc("/ui/states", ui.HandleStates(platform.DB)).Methods(http.MethodGet)
	r.HandleFunc("/ui/hubscore", ui.HandleHubScore(platform.DB)).Methods(http.MethodGet)
	r.HandleFunc("/ui/devices", ui.HandleDevices(platform.DB)).Methods(http.MethodGet)
	r.HandleFunc("/ui/agents", ui.HandleAgents(platform.DB)).Methods(http.MethodGet)
	r.HandleFunc("/ui/roi", ui.HandleROI(platform.DB)).Methods(http.MethodGet)
	r.HandleFunc("/ui/trigger", ui.HandleTriggerDemo).Methods(http.MethodPost)
	r.HandleFunc("/ui/unfreeze", ui.HandleUnfreeze(platform.DB)).Methods(http.MethodPost)
	r.HandleFunc("/ui/takedown", ui.HandleTakedown(platform.DB)).Methods(http.MethodPost)
	r.HandleFunc("/api/export/dossier", ui.HandleExportDossier(platform.DB)).Methods(http.MethodGet)
	r.HandleFunc("/ui/trigger-cinematic", ui.HandleTriggerCinematic).Methods(http.MethodPost)
	r.HandleFunc("/api/graph-data", ui.HandleGraphJSON(platform.DB)).Methods(http.MethodGet)
	r.HandleFunc("/ui/phone", ui.HandlePhoneScreen()).Methods(http.MethodGet)
	r.HandleFunc("/api/geo-data", ui.HandleGeoData(platform.DB)).Methods(http.MethodGet)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🚀 not-my-sim listening on :%s", port)
	log.Printf("   Endpoints: POST /webhooks/simswap | POST /events/transaction | POST /events/reset")
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("→ %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func notImplemented(phase string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		w.Write([]byte(`{"error":"not implemented","phase":"` + phase + `"}`))
	}
}

// handleResetDemo truncates all event/state tables and re-seeds account states
// so the demo can be restarted cleanly from a known state.
// This endpoint is for demo use only — never expose in production.
func handleResetDemo(w http.ResponseWriter, r *http.Request) {
	stmts := []string{
		`TRUNCATE TABLE swap_events, reset_events, transfer_events, graph_edges, pending_checks, device_edges`,
		`UPDATE account_state SET state = 'NORMAL', state_changed_at = NOW(), risk_window_expires_at = NULL, last_swap_request_id = NULL, is_frozen = FALSE`,
	}
	for _, stmt := range stmts {
		if _, err := platform.DB.Exec(stmt); err != nil {
			log.Printf("reset-demo: %v", err)
			http.Error(w, "reset failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	log.Println("🔄 Demo state reset to clean baseline")
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"reset","message":"all event tables truncated, account states set to NORMAL"}`))
}
