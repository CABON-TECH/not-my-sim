package ui

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/cabon-tech/not-my-sim/internal/actions"
	"github.com/cabon-tech/not-my-sim/internal/graph"
)

// ViewAccountState is the struct passed to states.html
type ViewAccountState struct {
	PhoneNumber string
	State       string
	Age         string
	DeviceID    string
	IsFrozen    bool
}

// ViewDevice tracks colluding hardware
type ViewDevice struct {
	DeviceID    string
	VictimCount int
}

// HandleIndex serves the main HTMX page.
func HandleIndex(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("web/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

// HandleStates serves the HTMX fragment for the account states table.
func HandleStates(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(`
			SELECT a.phone_number, a.state, a.state_changed_at, COALESCE(d.device_id, 'Unknown'), a.is_frozen
			FROM account_state a
			LEFT JOIN device_edges d ON a.phone_number = d.phone_number
			ORDER BY a.state_changed_at DESC 
			LIMIT 10`)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var states []ViewAccountState
		for rows.Next() {
			var s ViewAccountState
			var t time.Time
			if err := rows.Scan(&s.PhoneNumber, &s.State, &t, &s.DeviceID, &s.IsFrozen); err == nil {
				d := time.Since(t)
				if d < time.Minute {
					s.Age = "just now"
				} else {
					s.Age = d.Round(time.Second).String()
				}
				states = append(states, s)
			}
		}
		if err := rows.Err(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		tmpl, err := template.ParseFiles("web/states.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, states)
	}
}

// HandleDevices serves the HTMX fragment for the device collusion tracker.
func HandleDevices(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(`
			SELECT device_id, COUNT(DISTINCT phone_number) as v_count 
			FROM device_edges 
			GROUP BY device_id 
			ORDER BY v_count DESC
		`)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var devices []ViewDevice
		for rows.Next() {
			var d ViewDevice
			if err := rows.Scan(&d.DeviceID, &d.VictimCount); err == nil {
				devices = append(devices, d)
			}
		}
		if err := rows.Err(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		tmpl, err := template.ParseFiles("web/devices.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, devices)
	}
}

// ViewAgent tracks corrupt agents.
type ViewAgent struct {
	AgentID    string
	FraudCount int
}

// HandleAgents serves the HTMX fragment for the insider threat tracker.
func HandleAgents(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(`
			SELECT s.agent_id, COUNT(DISTINCT s.phone_number) as f_count
			FROM swap_events s
			JOIN account_state a ON s.phone_number = a.phone_number
			WHERE a.is_frozen = TRUE AND s.agent_id != 'system' AND s.agent_id != ''
			GROUP BY s.agent_id
			ORDER BY f_count DESC
		`)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var agents []ViewAgent
		for rows.Next() {
			var a ViewAgent
			if err := rows.Scan(&a.AgentID, &a.FraudCount); err == nil {
				agents = append(agents, a)
			}
		}
		if err := rows.Err(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		tmpl, err := template.ParseFiles("web/agents.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, agents)
	}
}

// HandleROI serves the Total KES Protected banner.
func HandleROI(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var total sql.NullFloat64
		err := db.QueryRow(`
			SELECT SUM(t.amount) 
			FROM transfer_events t 
			JOIN account_state a ON t.from_number = a.phone_number 
			WHERE a.is_frozen = TRUE
		`).Scan(&total)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		val := 0.0
		if total.Valid {
			val = total.Float64
		}

		// Simple HTML response for the counter
		w.Header().Set("Content-Type", "text/html")
		w.Write(fmt.Appendf(nil, `
			<div class="flex flex-col items-center justify-center p-6 bg-gradient-to-r from-emerald-900/40 to-teal-900/40 border border-emerald-800 rounded-xl shadow-[0_0_30px_rgba(16,185,129,0.15)] fade-in">
				<span class="text-emerald-400 font-bold tracking-widest uppercase text-sm mb-2 flex items-center gap-2">
					<span class="relative flex h-3 w-3">
					  <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
					  <span class="relative inline-flex rounded-full h-3 w-3 bg-emerald-500"></span>
					</span>
					Live Financial Impact (ROI)
				</span>
				<div class="text-5xl font-black text-transparent bg-clip-text bg-gradient-to-r from-emerald-400 to-teal-300 tabular-nums font-mono drop-shadow-lg">
					%s KES
				</div>
				<span class="text-gray-400 text-xs mt-3">Total funds automatically protected from confirmed fraud syndicates.</span>
			</div>
		`, formatMoney(val)))
	}
}

func formatMoney(amount float64) string {
	// Simple comma formatting for money
	str := fmt.Sprintf("%.0f", amount)
	n := len(str)
	if n <= 3 {
		return str
	}
	var res string
	for i, c := range str {
		if i > 0 && (n-i)%3 == 0 {
			res += ","
		}
		res += string(c)
	}
	return res
}

// HandleHubScore serves the HTMX fragment for the graph intelligence cards.
func HandleHubScore(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Query with minVictims=1 for the dashboard so we can see isolated fraud
		// before it upgrades to syndicate level.
		hubs, err := graph.QueryHubScore(db, 1, 24)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		tmpl, err := template.ParseFiles("web/hubscore.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, hubs)
	}
}

// HandleTriggerDemo runs the seed.go script in a background goroutine.
func HandleTriggerDemo(w http.ResponseWriter, r *http.Request) {
	go func() {
		log.Println("▶️  UI requested demo simulation trigger")
		cmd := exec.Command("go", "run", "scripts/seed.go")
		cmd.Env = append(cmd.Environ(), "SERVER_URL=http://localhost:8089")
		if err := cmd.Run(); err != nil {
			log.Printf("demo trigger err: %v", err)
		}
	}()
	w.WriteHeader(http.StatusAccepted)
}

// HandleUnfreeze manually overrides the risk engine and restores an account.
func HandleUnfreeze(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		phone := r.URL.Query().Get("phone")
		// Fix URL decoding: '+' is decoded as ' ', so we must restore it for E.164 format.
		if len(phone) > 0 && phone[0] == ' ' {
			phone = "+" + phone[1:]
		}

		if phone == "" {
			http.Error(w, "missing phone", http.StatusBadRequest)
			return
		}
		// Reset state to normal and unfreeze
		_, err := db.Exec(`
			UPDATE account_state 
			SET state = 'NORMAL', state_changed_at = NOW(), is_frozen = FALSE, risk_window_expires_at = NULL
			WHERE phone_number = $1`, phone)
		if err != nil {
			log.Printf("ui: unfreeze err: %v", err)
		}

		// Unlink device so it doesn't immediately re-trigger collusion limits for new demo attacks
		db.Exec(`DELETE FROM device_edges WHERE phone_number = $1`, phone)

		log.Printf("👨‍💻 UI ANALYST ACTION: Account %s unfrozen", phone)
		w.WriteHeader(http.StatusOK)
	}
}

// HandleTakedown freezes a mule and all connected victims.
func HandleTakedown(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mule := r.URL.Query().Get("mule")
		if len(mule) > 0 && mule[0] == ' ' {
			mule = "+" + mule[1:]
		}
		if mule == "" {
			http.Error(w, "missing mule", http.StatusBadRequest)
			return
		}

		// Freeze the mule
		db.Exec(`UPDATE account_state SET is_frozen = TRUE WHERE phone_number = $1`, mule)
		// Freeze all victims transferring to this mule
		db.Exec(`
			UPDATE account_state 
			SET is_frozen = TRUE 
			WHERE phone_number IN (
				SELECT from_number FROM graph_edges WHERE to_number = $1
			)`, mule)

		w.WriteHeader(http.StatusOK)
	}
}

// HandleExportDossier generates a JSON/text report for Law Enforcement.
func HandleExportDossier(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Disposition", `attachment; filename="NotMySim_Police_Dossier.txt"`)

		var dossier strings.Builder
		dossier.WriteString("===================================================\n")
		dossier.WriteString("      NOT-MY-SIM: LAW ENFORCEMENT FRAUD DOSSIER    \n")
		dossier.WriteString(fmt.Sprintf("      Generated: %s\n", time.Now().UTC().Format(time.RFC1123)))
		dossier.WriteString("===================================================\n\n")

		dossier.WriteString("[INSIDER THREATS - CORRUPT TELECOM AGENTS]\n")
		rowsA, _ := db.Query(`SELECT agent_id, COUNT(*) FROM swap_events WHERE agent_id != 'system' GROUP BY agent_id HAVING COUNT(*) >= 2`)
		for rowsA.Next() {
			var id string
			var count int
			rowsA.Scan(&id, &count)
			dossier.WriteString(fmt.Sprintf(" - Agent ID: %s (Authorized %d fraudulent swaps)\n", id, count))
		}
		if err := rowsA.Err(); err != nil {
			http.Error(w, "failed to read agent data", http.StatusInternalServerError)
			return
		}
		rowsA.Close()

		dossier.WriteString("\n[COMPROMISED HARDWARE (IMEI/MAC)]\n")
		rowsD, _ := db.Query(`SELECT device_id, COUNT(DISTINCT phone_number) FROM device_edges GROUP BY device_id HAVING COUNT(DISTINCT phone_number) >= 2`)
		for rowsD.Next() {
			var dev string
			var count int
			rowsD.Scan(&dev, &count)
			dossier.WriteString(fmt.Sprintf(" - Device ID: %s (Linked to %d victims)\n", dev, count))
		}
		if err := rowsD.Err(); err != nil {
			http.Error(w, "failed to read device data", http.StatusInternalServerError)
			return
		}
		rowsD.Close()

		dossier.WriteString("\n[SYNDICATE MULE ACCOUNTS (HUBS)]\n")
		rowsH, _ := db.Query(`SELECT to_number, SUM(weight) FROM graph_edges GROUP BY to_number HAVING SUM(weight) >= 20`)
		for rowsH.Next() {
			var mule string
			var weight int
			rowsH.Scan(&mule, &weight)
			dossier.WriteString(fmt.Sprintf(" - Mule Phone: %s (Total Risk Weight: %d)\n", mule, weight))
		}
		if err := rowsH.Err(); err != nil {
			http.Error(w, "failed to read mule account data", http.StatusInternalServerError)
			return
		}
		rowsH.Close()

		var total sql.NullFloat64
		db.QueryRow(`SELECT SUM(t.amount) FROM transfer_events t JOIN account_state a ON t.from_number = a.phone_number WHERE a.is_frozen = TRUE`).Scan(&total)
		val := 0.0
		if total.Valid {
			val = total.Float64
		}
		dossier.WriteString(fmt.Sprintf("\n[TOTAL FUNDS PROTECTED/FROZEN]\n - %.2f KES\n", val))

		dossier.WriteString("\n===================================================\nEND OF REPORT\n")
		w.Write([]byte(dossier.String()))
	}
}

// HandleTriggerCinematic fires off the slow-motion demo script asynchronously.
func HandleTriggerCinematic(w http.ResponseWriter, r *http.Request) {
	go func() {
		cmd := exec.Command("go", "run", "scripts/cinematic.go")
		cmd.Run() // Let it run in the background
	}()
	w.WriteHeader(http.StatusOK)
}

type GraphJSON struct {
	Nodes []NodeJSON `json:"nodes"`
	Edges []EdgeJSON `json:"edges"`
}
type NodeJSON struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Group string `json:"group"`
	Color string `json:"color"`
}
type EdgeJSON struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Dashes bool   `json:"dashes,omitempty"`
}

// HandleGraphJSON returns a JSON representation of the entire graph for vis.js.
func HandleGraphJSON(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var g GraphJSON
		g.Nodes = []NodeJSON{}
		g.Edges = []EdgeJSON{}

		// 1. Account Nodes (Only fetch nodes involved in fraud or suspected fraud to prevent graph lag)
		rowsA, _ := db.Query(`
			SELECT phone_number, is_frozen 
			FROM account_state 
			WHERE is_frozen = TRUE 
			   OR phone_number IN (SELECT from_number FROM graph_edges)
			   OR phone_number IN (SELECT to_number FROM graph_edges)
		`)
		for rowsA.Next() {
			var phone string
			var frozen bool
			rowsA.Scan(&phone, &frozen)
			color := "#10B981" // emerald
			if frozen {
				color = "#EF4444" // red
			}
			g.Nodes = append(g.Nodes, NodeJSON{
				ID:    phone,
				Label: phone,
				Group: "account",
				Color: color,
			})
		}
		if err := rowsA.Err(); err != nil {
			rowsA.Close()
			http.Error(w, "failed to read account nodes", http.StatusInternalServerError)
			return
		}
		rowsA.Close()

		// 2. Transfer Edges
		rowsE, _ := db.Query(`SELECT from_number, to_number FROM graph_edges`)
		mules := make(map[string]bool)
		for rowsE.Next() {
			var from, to string
			rowsE.Scan(&from, &to)
			g.Edges = append(g.Edges, EdgeJSON{From: from, To: to})
			mules[to] = true
		}
		if err := rowsE.Err(); err != nil {
			rowsE.Close()
			http.Error(w, "failed to read transfer edges", http.StatusInternalServerError)
			return
		}
		rowsE.Close()

		// Add missing mule nodes
		for mule := range mules {
			found := false
			for _, n := range g.Nodes {
				if n.ID == mule {
					found = true
					break
				}
			}
			if !found {
				g.Nodes = append(g.Nodes, NodeJSON{
					ID:    mule,
					Label: "MULE:\n" + mule,
					Group: "mule",
					Color: "#F59E0B", // amber
				})
			}
		}

		// 3. Device Nodes and Edges
		rowsD, _ := db.Query(`SELECT phone_number, device_id FROM device_edges`)
		devices := make(map[string]bool)
		for rowsD.Next() {
			var phone, dev string
			rowsD.Scan(&phone, &dev)
			g.Edges = append(g.Edges, EdgeJSON{From: phone, To: dev, Dashes: true})
			devices[dev] = true
		}
		if err := rowsD.Err(); err != nil {
			http.Error(w, "failed to read device edges", http.StatusInternalServerError)
			return
		}
		rowsD.Close()

		for dev := range devices {
			g.Nodes = append(g.Nodes, NodeJSON{
				ID:    dev,
				Label: "HW: " + dev,
				Group: "device",
				Color: "#6366F1", // indigo
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(g)
	}
}

// HandlePhoneScreen serves the interactive mobile phone simulator.
func HandlePhoneScreen() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFiles("web/phone.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, actions.LastSMS)
	}
}

// HandleGeoData returns the locations of recent fraudulent events.
func HandleGeoData(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Mock map of known locations to coords for demonstration
		coords := map[string][]float64{
			"Nairobi, KE": {-1.2921, 36.8219},
			"Mombasa, KE": {-4.0435, 39.6682},
			"Kisumu, KE":  {-0.0917, 34.7680},
		}

		rows, _ := db.Query(`SELECT location, COUNT(*) FROM transfer_events GROUP BY location`)

		var data []map[string]interface{}
		for rows.Next() {
			var loc string
			var count int
			rows.Scan(&loc, &count)
			if c, ok := coords[loc]; ok {
				data = append(data, map[string]interface{}{
					"location": loc,
					"lat":      c[0],
					"lng":      c[1],
					"count":    count,
				})
			}
		}
		if err := rows.Err(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		rows.Close()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
	}
}
