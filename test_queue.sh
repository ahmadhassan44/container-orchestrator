#!/bin/bash

# Test script to demonstrate job queuing feature
# This shows how jobs wait in queue instead of being rejected

GATEWAY_URL="http://localhost:3000"
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}╔═══════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║    Job Queuing Feature Demonstration          ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════════╝${NC}\n"

# Check if queue is enabled
echo -e "${YELLOW}Checking queue status...${NC}"
QUEUE_STATUS=$(curl -s $GATEWAY_URL/queue)
echo "$QUEUE_STATUS" | jq '.' 2>/dev/null || echo "$QUEUE_STATUS"
echo ""

ENABLED=$(echo "$QUEUE_STATUS" | grep -o '"enabled":[^,}]*' | cut -d: -f2 | tr -d ' ')

if [ "$ENABLED" != "true" ]; then
    echo -e "${RED}⚠ Job queuing is DISABLED${NC}"
    echo "To enable: Edit internal/gateway/scheduler.go and set ENABLE_JOB_QUEUE = true"
    exit 1
fi

echo -e "${GREEN}✓ Job queuing is ENABLED${NC}\n"

# Test 1: Saturate all workers
echo -e "${BLUE}═══════════════════════════════════════════════${NC}"
echo -e "${YELLOW}TEST 1: Saturating all workers${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════${NC}"
echo "Submitting 6 concurrent 70% jobs (3 workers × 2 jobs each)..."
echo ""

for i in {1..6}; do
    (curl -s -X POST $GATEWAY_URL/submit \
        -H "Content-Type: application/json" \
        -d '{"cpu_load": 70, "load_time": 8}' > /tmp/saturate_$i.json) &
done

sleep 2

echo -e "${BLUE}Checking queue status while jobs are running...${NC}"
QUEUE_STATUS=$(curl -s $GATEWAY_URL/queue)
QUEUE_SIZE=$(echo "$QUEUE_STATUS" | grep -o '"queue_size":[^,}]*' | cut -d: -f2 | tr -d ' ')
echo "$QUEUE_STATUS" | jq '.' 2>/dev/null || echo "$QUEUE_STATUS"
echo ""

if [ "$QUEUE_SIZE" -gt 0 ]; then
    echo -e "${GREEN}✓ Jobs are queued! Queue size: $QUEUE_SIZE${NC}"
else
    echo -e "${YELLOW}⚠ No jobs in queue (all workers handled load)${NC}"
fi

echo -e "\n${BLUE}Waiting for jobs to complete...${NC}"
wait

SUCCESS=0
FAILED=0
for i in {1..6}; do
    if [ -f /tmp/saturate_$i.json ]; then
        if grep -q "job_id" /tmp/saturate_$i.json; then
            ((SUCCESS++))
        else
            ((FAILED++))
        fi
    fi
done

echo ""
echo -e "${GREEN}✓ Results: $SUCCESS/6 jobs completed successfully${NC}"
if [ $FAILED -gt 0 ]; then
    echo -e "${RED}✗ $FAILED jobs failed${NC}"
fi

rm -f /tmp/saturate_*.json

# Test 2: Exceed capacity
echo -e "\n${BLUE}═══════════════════════════════════════════════${NC}"
echo -e "${YELLOW}TEST 2: Exceeding system capacity${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════${NC}"
echo "Submitting 15 concurrent jobs to test queue limits..."
echo ""

for i in {1..15}; do
    (curl -s -X POST $GATEWAY_URL/submit \
        -H "Content-Type: application/json" \
        -d '{"cpu_load": 60, "load_time": 5}' > /tmp/exceed_$i.json) &
done

sleep 1

echo -e "${BLUE}Queue status during overload:${NC}"
QUEUE_STATUS=$(curl -s $GATEWAY_URL/queue)
echo "$QUEUE_STATUS" | jq '.' 2>/dev/null || echo "$QUEUE_STATUS"
echo ""

echo -e "${BLUE}Waiting for all jobs to process...${NC}"
wait

SUCCESS=0
QUEUED=0
FAILED=0

for i in {1..15}; do
    if [ -f /tmp/exceed_$i.json ]; then
        CONTENT=$(cat /tmp/exceed_$i.json)
        if echo "$CONTENT" | grep -q "job_id"; then
            ((SUCCESS++))
        elif echo "$CONTENT" | grep -q "queue"; then
            ((QUEUED++))
        else
            ((FAILED++))
        fi
    fi
done

echo ""
echo -e "${GREEN}✓ Results:${NC}"
echo "  • Completed: $SUCCESS/15"
echo "  • Queued then completed: ~$QUEUED"
echo "  • Failed: $FAILED"

rm -f /tmp/exceed_*.json

# Test 3: Check final queue status
echo -e "\n${BLUE}═══════════════════════════════════════════════${NC}"
echo -e "${YELLOW}TEST 3: Final queue status${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════${NC}"

sleep 2

FINAL_STATUS=$(curl -s $GATEWAY_URL/queue)
echo "$FINAL_STATUS" | jq '.' 2>/dev/null || echo "$FINAL_STATUS"
echo ""

FINAL_QUEUE_SIZE=$(echo "$FINAL_STATUS" | grep -o '"queue_size":[^,}]*' | cut -d: -f2 | tr -d ' ')

if [ "$FINAL_QUEUE_SIZE" -eq 0 ]; then
    echo -e "${GREEN}✓ Queue is empty - all jobs processed${NC}"
else
    echo -e "${YELLOW}⚠ Queue still has $FINAL_QUEUE_SIZE jobs${NC}"
fi

# Summary
echo -e "\n${GREEN}╔═══════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║         Queue Feature Test Complete           ║${NC}"
echo -e "${GREEN}╚═══════════════════════════════════════════════╝${NC}\n"

echo -e "${BLUE}Key Observations:${NC}"
echo "  • Job queuing prevents immediate rejections"
echo "  • Queued jobs are processed when workers become available"
echo "  • Queue provides fair FIFO scheduling"
echo "  • System gracefully handles overload situations"
echo ""

echo -e "${YELLOW}To disable queuing:${NC}"
echo "  1. Edit internal/gateway/scheduler.go"
echo "  2. Set: const ENABLE_JOB_QUEUE = false"
echo "  3. Rebuild: ./start.sh"
echo ""
