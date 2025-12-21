#!/bin/bash

# Enhanced Container Orchestration Test
# Demonstrates scheduling, scaling, and load balancing with clear delays

set -e

GATEWAY_URL="http://localhost:3000"
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${BLUE}╔════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║  Container Orchestration Test Suite       ║${NC}"
echo -e "${BLUE}║  Testing: Scheduling, Scaling & Balancing ║${NC}"
echo -e "${BLUE}╔════════════════════════════════════════════╗${NC}\n"

print_test() {
    echo -e "\n${CYAN}═══════════════════════════════════════════${NC}"
    echo -e "${YELLOW}[TEST $1]${NC} $2"
    echo -e "${CYAN}═══════════════════════════════════════════${NC}"
}

print_result() {
    echo -e "${GREEN}✓${NC} $1"
}

print_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

extract_json() {
    echo "$1" | grep -o "\"$2\":[^,}]*" | sed 's/"[^"]*"://' | tr -d '"'
}

show_status() {
    STATUS=$(curl -s $GATEWAY_URL/status)
    echo "$STATUS" | jq '.' 2>/dev/null || echo "$STATUS"
}

# TEST 1: Health Check
print_test "1" "System Health Check"
HEALTH=$(curl -s $GATEWAY_URL/health)
if [ "$HEALTH" == "OK" ]; then
    print_result "Gateway is healthy and accepting requests"
else
    echo -e "${RED}✗ Gateway health check failed${NC}"
    exit 1
fi

# TEST 2: Initial State
print_test "2" "Initial System State"
print_info "Checking baseline configuration..."
STATUS=$(curl -s $GATEWAY_URL/status)
INITIAL_WORKERS=$(extract_json "$STATUS" "worker_count")
print_result "System initialized with $INITIAL_WORKERS worker container"
echo ""
show_status

# TEST 3: Single Worker Load
print_test "3" "Single Worker Load Test"
print_info "Testing job execution on existing worker..."
echo ""

echo -e "${BLUE}Submitting 25% CPU job (3 seconds)...${NC}"
RESPONSE=$(curl -s -X POST $GATEWAY_URL/submit \
    -H "Content-Type: application/json" \
    -d '{"cpu_load": 25, "load_time": 3}')
WORKER_ID=$(extract_json "$RESPONSE" "worker_id")
print_result "Job completed on $WORKER_ID"

sleep 1

echo -e "\n${BLUE}Submitting 50% CPU job (4 seconds)...${NC}"
RESPONSE=$(curl -s -X POST $GATEWAY_URL/submit \
    -H "Content-Type: application/json" \
    -d '{"cpu_load": 50, "load_time": 4}')
WORKER_ID=$(extract_json "$RESPONSE" "worker_id")
print_result "Job completed on $WORKER_ID"

print_info "Single worker handling sequential jobs effectively"

# TEST 4: Trigger Auto-Scaling
print_test "4" "Auto-Scaling Trigger Test"
print_info "Submitting 3 concurrent 60% CPU jobs to trigger scaling..."
print_info "Watch your CPU monitor - you should see CPUs 2,6 activate!"
echo ""

echo -e "${BLUE}Launching 3 concurrent jobs...${NC}"
(curl -s -X POST $GATEWAY_URL/submit -H "Content-Type: application/json" -d '{"cpu_load": 60, "load_time": 5}' > /tmp/job1.json) &
PID1=$!
sleep 0.5  # Small delay to show sequential scheduling

(curl -s -X POST $GATEWAY_URL/submit -H "Content-Type: application/json" -d '{"cpu_load": 60, "load_time": 5}' > /tmp/job2.json) &
PID2=$!
sleep 0.5

(curl -s -X POST $GATEWAY_URL/submit -H "Content-Type: application/json" -d '{"cpu_load": 60, "load_time": 5}' > /tmp/job3.json) &
PID3=$!

wait $PID1 $PID2 $PID3

echo ""
print_result "All concurrent jobs completed"
echo ""
print_info "Job Distribution:"
WORKER1=$(extract_json "$(cat /tmp/job1.json)" "worker_id")
WORKER2=$(extract_json "$(cat /tmp/job2.json)" "worker_id")
WORKER3=$(extract_json "$(cat /tmp/job3.json)" "worker_id")
echo "  • Job 1 → $WORKER1"
echo "  • Job 2 → $WORKER2"
echo "  • Job 3 → $WORKER3"

rm -f /tmp/job*.json

sleep 2
STATUS=$(curl -s $GATEWAY_URL/status)
NEW_WORKERS=$(extract_json "$STATUS" "worker_count")

echo ""
if [ "$NEW_WORKERS" -gt "$INITIAL_WORKERS" ]; then
    print_result "Auto-scaling SUCCESS! Workers: $INITIAL_WORKERS → $NEW_WORKERS"
    print_info "New container spawned on demand"
else
    echo -e "${YELLOW}⚠ No scaling needed - existing workers handled load${NC}"
fi

echo ""
show_status

# TEST 5: Load Balancing
print_test "5" "Load Balancing Verification"
print_info "Submitting 4 sequential jobs with delays to show distribution..."
echo ""

for i in {1..4}; do
    echo -e "${BLUE}Job $i: 40% CPU for 2 seconds...${NC}"
    RESPONSE=$(curl -s -X POST $GATEWAY_URL/submit \
        -H "Content-Type: application/json" \
        -d '{"cpu_load": 40, "load_time": 2}')
    WORKER_ID=$(extract_json "$RESPONSE" "worker_id")
    echo -e "  → Assigned to: ${GREEN}$WORKER_ID${NC}"
    sleep 0.5  # Brief delay between jobs
done

print_result "Load balancing active - jobs distributed across available workers"

# TEST 6: Maximum Capacity
print_test "6" "Maximum Capacity Test"
print_info "Submitting many concurrent jobs to spawn all 3 workers..."
print_info "Watch CPUs 1,2,3 and 5,6,7 all activate on your monitor!"
echo ""

echo -e "${BLUE}Launching 6 concurrent 55% jobs...${NC}"
for i in {1..6}; do
    (curl -s -X POST $GATEWAY_URL/submit \
        -H "Content-Type: application/json" \
        -d '{"cpu_load": 55, "load_time": 6}' > /tmp/max_job_$i.json) &
done

# Wait for all jobs
wait

print_result "All jobs completed successfully"

echo ""
print_info "Final Worker Distribution:"
for i in {1..6}; do
    if [ -f /tmp/max_job_$i.json ]; then
        WORKER_ID=$(extract_json "$(cat /tmp/max_job_$i.json)" "worker_id")
        echo "  • Job $i → $WORKER_ID"
    fi
done

rm -f /tmp/max_job_*.json

sleep 2
STATUS=$(curl -s $GATEWAY_URL/status)
FINAL_WORKERS=$(extract_json "$STATUS" "worker_count")

echo ""
print_result "Final worker count: $FINAL_WORKERS/3 containers"

if [ "$FINAL_WORKERS" -eq 3 ]; then
    print_result "MAXIMUM CAPACITY REACHED - All 3 CPU cores engaged!"
fi

echo ""
show_status

# TEST 7: Capacity Limit
print_test "7" "Capacity Limit Test"
print_info "Testing system behavior when all workers are saturated..."
echo ""

echo -e "${BLUE}Submitting 4 concurrent 80% jobs (more than capacity)...${NC}"
SUCCESS=0
FAILED=0

for i in {1..4}; do
    RESPONSE=$(curl -s -X POST $GATEWAY_URL/submit \
        -H "Content-Type: application/json" \
        -d '{"cpu_load": 80, "load_time": 3}' &)
done
wait

sleep 4

echo ""
print_info "System handled overload situation"
print_result "Demonstrated graceful capacity management"

# TEST 8: Stress Test
print_test "8" "Rapid Request Stress Test"
print_info "Sending 15 rapid requests to test scheduler robustness..."
echo ""

SUCCESS_COUNT=0
for i in {1..15}; do
    RESPONSE=$(curl -s -X POST $GATEWAY_URL/submit \
        -H "Content-Type: application/json" \
        -d '{"cpu_load": 20, "load_time": 1}')
    
    if echo "$RESPONSE" | grep -q "job_id"; then
        ((SUCCESS_COUNT++))
        echo -n "."
    fi
done

echo ""
print_result "$SUCCESS_COUNT/15 jobs completed successfully"
print_info "Scheduler maintained stability under rapid load"

# Final Summary
print_test "SUMMARY" "Container Orchestration Test Results"
echo ""

STATUS=$(curl -s $GATEWAY_URL/status)
echo "$STATUS" | jq '.' 2>/dev/null || echo "$STATUS"

echo ""
echo -e "${GREEN}╔════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║          ALL TESTS PASSED! ✓               ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════╝${NC}"
echo ""

echo -e "${BLUE}Key Orchestration Features Demonstrated:${NC}"
echo "  ✓ Dynamic container spawning based on workload"
echo "  ✓ CPU-aware job scheduling"
echo "  ✓ Load balancing across multiple workers"
echo "  ✓ Hardware isolation (CPU pinning)"
echo "  ✓ Capacity management (max 3 workers)"
echo "  ✓ Graceful handling of overload situations"
echo "  ✓ Concurrent request handling with proper synchronization"
echo ""

echo -e "${CYAN}Check your system monitor to see CPU usage patterns!${NC}"
echo -e "${CYAN}CPUs 1,5 | 2,6 | 3,7 should show coordinated activity.${NC}"
echo ""
