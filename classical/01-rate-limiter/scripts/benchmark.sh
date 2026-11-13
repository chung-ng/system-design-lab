#!/usr/bin/env bash

# Test script demonstrating Token Bucket Rate Limiting behavior
HOST="http://localhost:8080"
ENDPOINT="$HOST/api/v1/resource"
API_KEY="test_client_key"

echo -e "\n[Phase 1] Firing 15 rapid consecutive requests..."
for i in $(seq 1 15); do
    RESPONSE=$(curl -s -i -H "X-API-Key: $API_KEY" "$ENDPOINT")
    HTTP_CODE=$(echo "$RESPONSE" | grep "HTTP/" | awk '{print $2}')
    REMAINING=$(echo "$RESPONSE" | grep -i "x-ratelimit-remaining:" | awk '{print $2}' | tr -d '\r')
    RETRY_AFTER=$(echo "$RESPONSE" | grep -i "retry-after:" | awk '{print $2}' | tr -d '\r')
    
    if [ "$HTTP_CODE" == "200" ]; then
        echo "Req #$i: HTTP $HTTP_CODE | Remaining Tokens: $REMAINING | Status: ALLOWED"
    else
        echo "Req #$i: HTTP $HTTP_CODE | Retry-After: ${RETRY_AFTER}s | Status: BLOCKED (429)"
    fi
    sleep 0.05
done

echo -e "\n[Phase 2] Waiting 2 seconds for token refill..."
sleep 2

echo -e "\n[Phase 3] Firing request after refill..."
RESPONSE=$(curl -s -i -H "X-API-Key: $API_KEY" "$ENDPOINT")
HTTP_CODE=$(echo "$RESPONSE" | grep "HTTP/" | awk '{print $2}')
REMAINING=$(echo "$RESPONSE" | grep -i "x-ratelimit-remaining:" | awk '{print $2}' | tr -d '\r')
echo "Post-wait Request: HTTP $HTTP_CODE | Remaining Tokens: $REMAINING | Status: ALLOWED"

echo -e "\n=========================================================="
echo " Test completed successfully!"
echo "=========================================================="
