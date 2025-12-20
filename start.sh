#!/bin/bash

# Build worker image
echo "Building worker Docker image..."
docker build -f Dockerfile.worker -t container-orchestrator-worker:latest .

if [ $? -eq 0 ]; then
    echo "Worker image built successfully"
else
    echo "Failed to build worker image"
    exit 1
fi

# Run gateway
echo ""
echo "Starting gateway..."
go run ./cmd/gateway/main.go
