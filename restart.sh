#!/bin/bash
set -u

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
WEB_DIR="$ROOT_DIR/web"
BACKEND_PID_FILE="/tmp/nofx_runtime.pid"
FRONTEND_PID_FILE="/tmp/nofx_frontend.pid"
BACKEND_LOG_FILE="$ROOT_DIR/nofx.log"
FRONTEND_LOG_FILE="$ROOT_DIR/web-dev.log"

cd "$ROOT_DIR" || exit 1

kill_port() {
  local port="$1"
  lsof -t -i :"$port" | xargs kill -9 2>/dev/null || true
}

kill_pid_file() {
  local pid_file="$1"
  if [ -f "$pid_file" ]; then
    local old_pid
    old_pid="$(cat "$pid_file" 2>/dev/null || true)"
    if [ -n "${old_pid:-}" ]; then
      kill -9 "$old_pid" 2>/dev/null || true
    fi
    rm -f "$pid_file"
  fi
}

wait_http_ok() {
  local url="$1"
  local retries="$2"
  local sleep_s="$3"
  local i
  for i in $(seq 1 "$retries"); do
    if curl -sS "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep "$sleep_s"
  done
  return 1
}

echo "Stopping old backend/frontend..."
kill_port 8080
kill_port 3000
kill_pid_file "$BACKEND_PID_FILE"
kill_pid_file "$FRONTEND_PID_FILE"

# Keep this as a fallback cleanup in case historical process exists.
pkill nofx 2>/dev/null || true

echo "Building backend (nofx)..."
export GOCACHE="${GOCACHE:-/tmp/go-build}"
mkdir -p "$GOCACHE"
if ! go build -o nofx; then
  echo "Build failed."
  exit 1
fi

echo "Starting backend (8080)..."
nohup ./nofx >"$BACKEND_LOG_FILE" 2>&1 < /dev/null &
BACKEND_PID=$!
echo "$BACKEND_PID" > "$BACKEND_PID_FILE"

if ! wait_http_ok "http://localhost:8080/api/health" 30 0.5; then
  echo "Backend failed to become healthy. Last logs:"
  tail -n 140 "$BACKEND_LOG_FILE" || true
  exit 1
fi
echo "Backend is healthy. PID: $BACKEND_PID"

if [ ! -d "$WEB_DIR" ]; then
  echo "Web directory not found: $WEB_DIR"
  exit 1
fi

if ! command -v npm >/dev/null 2>&1; then
  echo "npm not found. Cannot start frontend."
  exit 1
fi

echo "Starting frontend (3000)..."
cd "$WEB_DIR" || exit 1
nohup npm run dev -- --host localhost --port 3000 >"$FRONTEND_LOG_FILE" 2>&1 < /dev/null &
FRONTEND_PID=$!
echo "$FRONTEND_PID" > "$FRONTEND_PID_FILE"

if ! wait_http_ok "http://localhost:3000" 40 0.5; then
  echo "Frontend failed to become ready. Last logs:"
  tail -n 140 "$FRONTEND_LOG_FILE" || true
  exit 1
fi

# Verify the Vite proxy can reach backend to prevent /traders server error.
if ! wait_http_ok "http://localhost:3000/api/health" 20 0.5; then
  echo "Frontend started, but proxy to backend is not healthy."
  echo "Backend log tail:"
  tail -n 80 "$BACKEND_LOG_FILE" || true
  echo "Frontend log tail:"
  tail -n 80 "$FRONTEND_LOG_FILE" || true
  exit 1
fi

echo "Done."
echo "Frontend: http://localhost:3000"
echo "Backend:  http://localhost:8080"
echo "Proxy:    http://localhost:3000/api/health"
echo "Backend PID:  $BACKEND_PID"
echo "Frontend PID: $FRONTEND_PID"
