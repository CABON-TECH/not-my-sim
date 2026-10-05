# Not-My-SIM

A real-time fraud prevention engine that detects SIM-swap attacks by correlating telecom and financial event streams. It combines a state machine, graph-based syndicate detection, and geo-velocity analysis to identify and block fraudulent mobile money transactions before funds leave the network.

---

## Overview

SIM-swap fraud succeeds because banks and telecoms operate in isolation. The bank sees a valid PIN and approves the transfer. The telco recorded a SIM registration change hours earlier but never told the bank. Not-My-SIM sits between both systems, ingesting event streams from each side and evaluating them as a unified timeline.

When the engine detects a suspicious sequence — a SIM swap followed by a credential reset followed by an outbound transfer within a configurable risk window — it scores the transaction and takes automated action: an interactive SMS challenge, an account freeze, or an outbound voice alert.

---

## How It Works

### State Machine (Complex Event Processing)

Every monitored phone number moves through a finite set of states:

```
NORMAL -> SWAP_DETECTED -> CREDENTIAL_RESET_POST_SWAP -> TRANSFER_POST_RESET
```

Events that arrive out of sequence are dropped. A transfer that arrives while an account is in `NORMAL` state is ignored. A transfer that arrives while the account is in `CREDENTIAL_RESET_POST_SWAP` state triggers the scoring engine. This removes the need for complex rule sets — the state itself encodes the risk context.

### Scoring Engine

Once a critical event is processed, the engine evaluates a risk score from 0 to 100 using the following signals:

- **CEP state weight** — base score derived from the current state transition
- **Geo-velocity** — distance and time delta between the reset location and the transfer location; flags physically impossible travel
- **Device collusion** — whether the same hardware IMEI was used to reset credentials across multiple victim accounts
- **Graph hub score** — whether the transfer recipient is a known mule hub (multiple victims routing funds to the same number)

The score determines the response:

| Score | Action |
|---|---|
| 0 - 64 | No action |
| 65 - 84 | Interactive 2FA SMS dispatched to victim |
| 85 - 100 | Account frozen immediately, voice alert triggered |

### Africa's Talking Integration

- **SMS Webhooks** — When a transaction scores in the 65-84 range, an SMS is sent to the victim's number with a reply prompt. The victim replies `2` to block the transfer. Africa's Talking fires the reply to `/webhooks/sms`, which freezes the account in the database.
- **Voice API** — Maximum-risk events trigger an outbound call via the AT Voice API. The call plays a spoken fraud alert when answered.
- **SIM Swap Node** — The engine verifies SIM swap history directly against the AT telco infrastructure on startup as a connectivity gate check.

### Syndicate Topology (Graph Analysis)

Every confirmed fraud transaction writes a directed edge to a graph table: `victim -> mule`. The engine periodically queries this graph for hub nodes — recipient accounts that appear as the destination for multiple victims. When a hub is identified, a syndicate report is generated and all connected victim accounts are eligible for bulk freeze.

Device edges are tracked separately. If the same hardware IMEI appears in credential reset events across two or more accounts, both accounts are flagged regardless of their individual scores.

### Insider Threat Tracking

SIM swaps in Kenya are processed by registered telco agents. Every swap event carries the authorizing agent ID. The engine groups confirmed-fraud swap events by agent ID. Agents appearing across multiple fraudulent swaps are surfaced in the operations dashboard as insider threat candidates.

---

## Architecture

| Layer | Technology |
|---|---|
| Backend | Go — event bus, state machine, HTTP routing |
| Database | PostgreSQL — relational graph edges, state persistence |
| Frontend | HTMX polling, Tailwind CSS |
| Visualizations | Vis.js (topology graph), Leaflet.js + OpenStreetMap (geo map) |
| Telecom | Africa's Talking SMS, Voice, and SIM Swap APIs |

---

## Local Setup

### Prerequisites

- Go 1.21 or later
- PostgreSQL
- ngrok (required to receive Africa's Talking SMS reply webhooks locally)

### Database

```bash
psql -U postgres -c "CREATE USER notmysim WITH PASSWORD 'notmysim_secret';"
psql -U postgres -c "CREATE DATABASE notmysim OWNER notmysim;"
```

### Environment

Create a `.env` file in the project root:

```env
PORT=8089
DATABASE_URL=postgres://notmysim:notmysim_secret@localhost:5433/notmysim?sslmode=disable

AT_USERNAME=sandbox
AT_API_KEY=your_api_key_here
AT_SMS_SHORTCODE=20880
AT_VOICE_NUMBER=
AT_WHITELIST=+254700000000
```

`AT_WHITELIST` accepts a comma-separated list of phone numbers that are permitted to receive live SMS and voice calls. Any number not on the list will be processed internally but will not trigger a real API call. Leave it empty to allow all numbers.

### Running

```bash
go build ./... && go run ./cmd/server
```

The dashboard is available at `http://localhost:8089`.

### Webhooks

Start ngrok in a separate terminal:

```bash
ngrok http 8089
```

Set the ngrok URL as the SMS callback in your Africa's Talking dashboard under **SMS > SMS Callback URLs > Incoming Messages**:

```
https://<your-ngrok-subdomain>.ngrok-free.app/webhooks/sms
```

---

## Simulation

The project includes a cinematic simulation script that injects a staged three-account attack sequence at a pace suitable for demonstration:

```bash
go run scripts/cinematic.go
```

The sequence covers three distinct attack profiles: maximum-risk immediate freeze, syndicate hub detection, and the grey-area interactive SMS challenge. A reset script is included to truncate state between runs:

```bash
./scripts/reset_demo.sh
```

---

## License

MIT
