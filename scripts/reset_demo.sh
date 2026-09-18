#!/bin/bash
# reset_demo.sh - Helper script to easily wipe the demo data between judging sessions.

URL=${SERVER_URL:-"http://localhost:8089"}

echo "🔄 Sending reset signal to not-my-sim server at $URL..."

RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$URL/admin/reset-demo")
BODY=$(echo "$RESPONSE" | head -n -1)
CODE=$(echo "$RESPONSE" | tail -n 1)

if [ "$CODE" == "200" ]; then
    echo "✅ Demo state cleanly reset!"
    echo "   Database tables truncated and accounts set back to NORMAL."
else
    echo "❌ Failed to reset (HTTP $CODE): $BODY"
    echo "   Is the server running?"
fi
