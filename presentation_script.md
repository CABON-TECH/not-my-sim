# 🎤 Not-My-SIM: Pitch & Q&A Guide

## ⏱️ The 3-Minute Pitch Script

### [Slide 1: The Hook]
**"Imagine waking up to find your phone has no service. You assume it's just a network glitch. By the time you find Wi-Fi two hours later, your mobile money wallet is completely empty. This is SIM Swap fraud, and it's devastating."**

"The problem is that traditional banking systems only see the money moving. They are completely blind to the telecom signals happening behind the scenes—like SIM registrations changing or devices moving across the country."

### [Slide 2: The Solution]
"We built **Not-My-SIM**: A real-time fraud prevention engine that bridges the gap between Telecom networks and Fintech platforms."

"Instead of looking at isolated transactions, our engine uses Complex Event Processing to track the entire lifecycle of an attack: from the moment a corrupt agent swaps a SIM, to the PIN reset, to the final fraudulent transfer."

### [Slide 3: Africa's Talking Integration]
"We heavily integrated **Africa's Talking APIs** as our critical defense layer:
1. **Interactive SMS Webhooks:** When our engine detects a 'grey area' transaction, it fires an AT SMS to the victim. The victim can literally reply '2' to instantly trigger a webhook that freezes their funds mid-flight.
2. **Automated Robocalls:** For 100/100 maximum risk attacks, we trigger AT Voice APIs to urgently wake up the victim.
3. **SIM Node Verification:** Connecting directly to telco infrastructure to verify if a SIM was swapped in the last 72 hours."

### [Slide 4: The Live Demo (Switch to Dashboard)]
"Let's look at the live operations dashboard. *(Click Reset, then Cinematic Demo)*."
"We are injecting a live, multi-stage attack. Watch the **Real-time Geo-Velocity Map** track the impossible travel. Look at the **Spiderweb Graph** dynamically drawing the criminal syndicate's network in real-time."
"Notice Account C—it scored 70 points. It's suspicious, but not guaranteed fraud. Our system just dispatched an interactive SMS to my real phone..." *(Hold up your phone, show the SMS, reply '2'. Point to the dashboard as the account instantly freezes and the ROI tracker jumps up).*

### [Slide 5: Conclusion]
"With Not-My-SIM, we don't just log fraud after it happens. We map the syndicate, catch the corrupt insiders, and freeze the funds before the money ever leaves the network. Thank you."

---

## ⚖️ Judge Q&A: Anticipated Questions & Answers

### 1. "Won't running all these checks slow down mobile money transfers?"
**Your Answer:** "No. We built the core engine in Go (Golang), which is incredibly fast. The state machine evaluation happens in memory in microseconds. The heavy graph computations (Syndicate mapping) happen asynchronously. It does not block the payment gateway."

### 2. "Why use PostgreSQL for the Graph mapping instead of a dedicated graph database like Neo4j?"
**Your Answer:** "To reduce latency and operational overhead. For real-time fraud detection, we only need to look 1 to 2 hops away to find 'Mule Hubs'. We wrote highly optimized SQL self-joins that can calculate a Hub Score in milliseconds without needing to maintain and sync a separate Graph database."

### 3. "How do you handle False Positives? What if I travel to Mombasa, buy a new phone, and reset my PIN?"
**Your Answer:** "That's exactly why we built the Interactive 2FA! A PIN reset on a new device in a new city will give you a moderate risk score (e.g., 70/100). The system won't instantly freeze your money—instead, it uses the Africa's Talking API to send you an SMS asking if you initiated the transfer. You just reply '1' to approve it. We only do zero-touch automated freezes for 100/100 scores (e.g., a SIM Swap + Impossible Travel + Transferring to a known Syndicate Hub)."

### 4. "How would a Bank or Telco actually integrate this?"
**Your Answer:** "It's built as an API-first Event Bus. A Telco pushes webhooks to our `/events/simswap` endpoint, and a Bank pushes webhooks to our `/events/transaction` endpoint. Our engine sits in the middle, aggregates the data, and fires webhooks back to the bank to halt transactions if the risk threshold is crossed."

### 5. "I saw 'Insider Threats' on the dashboard. How does that work?"
**Your Answer:** "When a SIM swap happens, it's often facilitated by a corrupt telecom agent. Because we map the data relationally, we group fraudulent transactions by the `AgentID` that authorized the original SIM swap. If an agent authorizes 3 SIM swaps that all lead to fraud, they light up on our dashboard for immediate termination."

### 6. "What happens if the attacker has already stolen the victim's phone entirely?"
**Your Answer:** "If the physical phone is stolen (not just a SIM swap), the SMS 2FA goes to the thief. However, our Geo-Velocity tracker and Device Fingerprinting (IMEI) will still flag the transaction if they try to move the SIM to a burner phone or if they route the money to a known mule network. If the score hits 85+, we bypass the SMS entirely and just freeze the account."
