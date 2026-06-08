#!/bin/bash
set -e

echo "=== AI Gateway Deployment ==="

# 1. Check for config.yaml
if [ ! -f config.yaml ]; then
    echo "[INFO] config.yaml not found, copying from config.yaml.example"
    cp config.yaml.example config.yaml
    echo "[WARN] Please edit config.yaml with your MySQL credentials and routes, then re-run this script."
    exit 1
fi

# 2. Build Linux binary (if source is newer)
if [ ! -f aigateway ] || [ aigateway -ot main.go ]; then
    echo "[INFO] Building aigateway for Linux..."
    GOOS=linux GOARCH=amd64 go build -o aigateway .
else
    echo "[INFO] aigateway binary is up to date, skipping build."
fi

# 3. Start service
echo "[INFO] Starting gateway..."
docker compose up -d --build

# 4. Wait for ready
echo "[INFO] Waiting for gateway to be ready..."
for i in $(seq 1 30); do
    if curl -sf http://localhost:${SERVER_PORT:-8080}/admin > /dev/null 2>&1; then
        echo "[OK] Gateway is ready."
        echo "[OK] Admin UI: http://localhost:${SERVER_PORT:-8080}/admin"
        exit 0
    fi
    sleep 2
done

echo "[ERROR] Gateway failed to start within 60s. Check logs: docker compose logs aigateway"
exit 1
