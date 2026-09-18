# 🛡️ Not-My-SIM

> **Africa's Talking Telecom Innovation Hackathon**
> 
> A real-time, event-driven telecom fraud prevention engine that detects and neutralizes SIM-swap syndicates before the money leaves the network.

## ⚠️ The Problem
Telecom fraud (specifically SIM Swapping combined with Mobile Money credential theft) is costing millions. Traditional banking systems only see the money moving—they lack the critical telecom context (SIM registration changes, device IMEI swaps, geolocation). By the time a victim realizes their phone lost signal, the syndicate has already drained their accounts.

## 🚀 The Solution
**Not-My-SIM** sits at the intersection of Telecom and Fintech. It ingests high-velocity event streams (SIM Swaps, PIN Resets, Money Transfers) and evaluates them through a multi-layered risk engine to detect impossible travel, device collusion, and graph-based syndicate hubs.

When critical fraud is detected, the system autonomously triggers **Africa's Talking APIs** to execute Interactive SMS 2FA, automated Robocalls, and instant account freezes.

---

## ✨ Core Features

*   **⚡ Real-Time Complex Event Processing (CEP)**
    A Go-powered state machine that tracks the lifecycle of an attack (e.g., `SWAP_DETECTED` → `CREDENTIAL_RESET` → `TRANSFER`).
*   **🕸️ Syndicate Topology Mapping (Graph Hubs)**
    Maps relationships between compromised devices, victims, and recipient accounts to identify "Mule Hubs" used by organized crime.
*   **🌍 Geo-Velocity & Device Collusion Tracker**
    Flags "impossible travel" (e.g., a PIN reset in Kisumu followed by a transfer in Mombasa 5 minutes later) and tracks suspicious IMEIs used across multiple accounts.
*   **📱 Interactive 2FA SMS (Africa's Talking)**
    If a transaction falls in a "grey area" (Score: 65-84), the engine dispatches an interactive AT SMS. The victim can reply `2` to instantly freeze their account via webhook.
*   **🤖 AI Analyst Synthesis**
    Automatically generates forensic, natural-language intelligence reports when a syndicate is detected, ready for law enforcement export.
*   **🕵️ Insider Threat Detection**
    Correlates fraudulent SIM swaps back to the specific Telecom Agent ID who authorized them.

---

## 🛠️ Architecture

*   **Backend:** Go (Golang) event-bus, state-machine, and routing.
*   **Database:** PostgreSQL (Relational mapping for Graph/Hub logic).
*   **Frontend:** HTMX (WebSockets/Polling), TailwindCSS.
*   **Visualizations:** Vis.js (Network Topology Map), Leaflet.js (Geo-Map).
*   **Telecom Integration:** Africa's Talking API (SMS, Voice, SIM Swap, Webhooks).

---

## 💻 Local Setup & Quick Start

### 1. Prerequisites
*   Go 1.21+
*   PostgreSQL
*   [ngrok](https://ngrok.com/) (For receiving Africa's Talking webhooks)

### 2. Environment Setup
Create a `.env` file in the project root:
```env
PORT=8089
DATABASE_URL=postgres://notmysim:notmysim_secret@localhost:5433/notmysim?sslmode=disable

# Africa's Talking Credentials
AT_USERNAME=sandbox
AT_API_KEY=your_sandbox_api_key
AT_SMS_SHORTCODE=20880
```

### 3. Start the Engine
```bash
# Start the Go server
go build ./... && go run ./cmd/server
```
Visit `http://localhost:8089` to view the Live Operations Dashboard.

### 4. Wire up Africa's Talking Webhooks
To use the Interactive SMS 2FA with a real phone:
1. Start ngrok: `ngrok http 8089`
2. Go to your Africa's Talking Dashboard.
3. Under **SMS > SMS Callback URLs > Incoming Messages**, paste:
   `https://<your-ngrok-url>/webhooks/sms`

---

## 🎬 Running the Hackathon Demo
We built a custom "Cinematic Demo Mode" for the judges!
1. Click **Reset Demo** in the dashboard to truncate the database.
2. Click **🎬 Cinematic Demo Mode**.
3. Watch the dashboard as the backend slowly injects simulated attacks:
    *   **Account A:** Maximum Risk (100/100). Triggers an instant account freeze and logs an AT Robocall.
    *   **Account B:** Syndicate detection. Watch the Spiderweb Map draw the connections and the AI Analyst generate a report.
    *   **Account C:** Grey Area Risk (70/100). Dispatches a real Africa's Talking 2FA SMS to your phone. Reply "2" to watch the dashboard instantly freeze the account!

---
*Built with ❤️ for the Africa's Talking Telecom Innovation Hackathon*
