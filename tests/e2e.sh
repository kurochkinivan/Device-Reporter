#!/bin/bash

# E2E acceptance tests for device-reporter
# Run from root directory: ./tests/e2e.sh

set -o pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

IMAGE_NAME="device-reporter-e2e"
PG_CONTAINER="device-reporter-e2e-postgres"
PG_USER="postgres"
PG_PASS="postgres"
PG_DB="device_reporter_test"
PG_PORT="5433"

testNumber=0
failedTests=0

APP_PID=""
WATCH_DIR="$(pwd)/tests/tmp/watch"
REPORTS_DIR="$(pwd)/tests/tmp/reports"
TEST_CONFIG="$(pwd)/tests/tmp/config.yaml"

# ──────────────────────────────────────────
# Helpers
# ──────────────────────────────────────────

function log_section {
  echo ""
  echo -e "${YELLOW}══════════════════════════════════════${NC}"
  echo -e "${YELLOW} $1${NC}"
  echo -e "${YELLOW}══════════════════════════════════════${NC}"
}

function assertExitCode {
  local expected=$1
  local actual=$2

  if [ "$expected" -ne "$actual" ]; then
    echo -e "${RED}✗ Test #${testNumber} FAILED — expected exit code $expected, got $actual${NC}"
    failedTests=$((failedTests+1))
  else
    echo -e "${GREEN}✓ Test #${testNumber} PASSED${NC}"
  fi
  testNumber=$((testNumber+1))
}

function assertHttpCode {
  local expected=$1
  local actual=$2

  if [ "$expected" != "$actual" ]; then
    echo -e "${RED}✗ Test #${testNumber} FAILED — expected HTTP $expected, got $actual${NC}"
    failedTests=$((failedTests+1))
  else
    echo -e "${GREEN}✓ Test #${testNumber} PASSED — HTTP $actual${NC}"
  fi
  testNumber=$((testNumber+1))
}

function assertFileExists {
  local path=$1
  if [ -f "$path" ]; then
    echo -e "${GREEN}✓ Test #${testNumber} PASSED — file exists: $path${NC}"
  else
    echo -e "${RED}✗ Test #${testNumber} FAILED — file not found: $path${NC}"
    failedTests=$((failedTests+1))
  fi
  testNumber=$((testNumber+1))
}

function assertDbCount {
  local query=$1
  local expected=$2
  local actual
  actual=$(docker exec "$PG_CONTAINER" psql -U "$PG_USER" -d "$PG_DB" -tAc "$query")
  actual=$(echo "$actual" | tr -d '[:space:]')

  if [ "$actual" = "$expected" ]; then
    echo -e "${GREEN}✓ Test #${testNumber} PASSED — query returned $actual (expected $expected)${NC}"
  else
    echo -e "${RED}✗ Test #${testNumber} FAILED — query returned $actual (expected $expected)${NC}"
    failedTests=$((failedTests+1))
  fi
  testNumber=$((testNumber+1))
}

function assertDbValue {
  local query=$1
  local expected=$2
  local actual
  actual=$(docker exec "$PG_CONTAINER" psql -U "$PG_USER" -d "$PG_DB" -tAc "$query")
  actual=$(echo "$actual" | tr -d '[:space:]')

  if [ "$actual" = "$expected" ]; then
    echo -e "${GREEN}✓ Test #${testNumber} PASSED — got '$actual'${NC}"
  else
    echo -e "${RED}✗ Test #${testNumber} FAILED — expected '$expected', got '$actual'${NC}"
    failedTests=$((failedTests+1))
  fi
  testNumber=$((testNumber+1))
}

function verifyAllTestsPassed {
  echo ""
  echo "Total tests: $testNumber"
  echo "Failed: $failedTests"

  if [ "$failedTests" -ne 0 ]; then
    echo -e "${RED}Some tests have failed!${NC}"
    exit 1
  else
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
  fi
}

# очищает содержимое watch и reports между блоками тестов
function reset_dirs {
  rm -f "$WATCH_DIR"/*
  rm -f "$REPORTS_DIR"/*
}

function cleanup_db {
  docker exec "$PG_CONTAINER" psql -U "$PG_USER" -d "$PG_DB" \
    -c "TRUNCATE files, devices RESTART IDENTITY CASCADE;" > /dev/null
}

function run_app {
  local timeout=${1:-3}
  docker run --rm \
    --network host \
    -v "$TEST_CONFIG":/app/config.yaml \
    -v "$WATCH_DIR":/watch \
    -v "$REPORTS_DIR":/reports \
    "$IMAGE_NAME" /bin/device_reporter \
    --config=/app/config.yaml &

  APP_PID=$!
  sleep "$timeout"
  kill "$APP_PID" 2>/dev/null || true
  wait "$APP_PID" 2>/dev/null || true
  APP_PID=""
}

function wait_for_http {
  local url=$1
  local attempts=0
  echo "Waiting for HTTP server..."
  until curl -s -o /dev/null "$url" 2>/dev/null; do
    attempts=$((attempts+1))
    if [ $attempts -ge 20 ]; then
      echo -e "${RED}HTTP server did not start in time${NC}"
      return 1
    fi
    sleep 0.5
  done
  echo "HTTP server is ready"
}

# ──────────────────────────────────────────
# Cleanup
# ──────────────────────────────────────────

function cleanup {
  echo ""
  echo "Cleaning up..."
  if [ -n "$APP_PID" ]; then
    kill "$APP_PID" 2>/dev/null || true
    wait "$APP_PID" 2>/dev/null || true
  fi
  docker stop "$PG_CONTAINER" 2>/dev/null || true
  docker rm "$PG_CONTAINER" 2>/dev/null || true
  rm -rf "$(pwd)/tests/tmp"
  docker rmi "$IMAGE_NAME" 2>/dev/null || true
}

trap cleanup EXIT

# убиваем старый контейнер если остался с прошлого запуска
docker stop "$PG_CONTAINER" 2>/dev/null || true
docker rm "$PG_CONTAINER" 2>/dev/null || true

# ──────────────────────────────────────────
# Setup
# ──────────────────────────────────────────

log_section "Setup"

echo "Building Docker image: $IMAGE_NAME"
docker build -q -t "$IMAGE_NAME" .

echo "Starting PostgreSQL container..."
docker run -d \
  --name "$PG_CONTAINER" \
  -e POSTGRES_USER="$PG_USER" \
  -e POSTGRES_PASSWORD="$PG_PASS" \
  -e POSTGRES_DB="$PG_DB" \
  -e PGDATA=/var/lib/postgresql/data/pgdata \
  -p "$PG_PORT":5432 \
  postgres:18-alpine > /dev/null

echo "Waiting for PostgreSQL to be ready..."
until docker exec "$PG_CONTAINER" pg_isready -U "$PG_USER" -d "$PG_DB" > /dev/null 2>&1; do
  sleep 0.5
done
echo "PostgreSQL is ready"

echo "Applying migrations..."
docker run --rm \
  --network host \
  "$IMAGE_NAME" /bin/migrate \
  --type=up \
  --username="$PG_USER" \
  --password="$PG_PASS" \
  --host=localhost \
  --port="$PG_PORT" \
  --db="$PG_DB"

mkdir -p "$WATCH_DIR" "$REPORTS_DIR"

cat > "$TEST_CONFIG" <<EOF
app:
  scan_interval: 1s
  watch_dir: /watch
  reports_dir: /reports

postgresql:
  host: localhost
  port: $PG_PORT
  username: $PG_USER
  password: $PG_PASS
  dbname: $PG_DB

http:
  host: 0.0.0.0
  port: 8080
  idle_timeout: 5m
  read_timeout: 10s
  write_timeout: 10s
EOF

# ──────────────────────────────────────────
# Negative tests — флаги
# ──────────────────────────────────────────

log_section "Negative tests — flags validation"

echo "Test #$testNumber: missing required flag --watch-dir"
docker run --rm "$IMAGE_NAME" /bin/device_reporter \
  --pg-host=localhost --pg-port=5432 --pg-username=postgres \
  --pg-password=postgres --pg-dbname=test \
  --reports-dir=/tmp --scan-interval=30s > /dev/null 2>&1
assertExitCode 1 $?

echo "Test #$testNumber: missing required flag --pg-host"
docker run --rm "$IMAGE_NAME" /bin/device_reporter \
  --watch-dir=/tmp --reports-dir=/tmp --scan-interval=30s \
  --pg-port=5432 --pg-username=postgres \
  --pg-password=postgres --pg-dbname=test > /dev/null 2>&1
assertExitCode 1 $?

echo "Test #$testNumber: missing required flag --scan-interval"
docker run --rm "$IMAGE_NAME" /bin/device_reporter \
  --watch-dir=/tmp --reports-dir=/tmp \
  --pg-host=localhost --pg-port=5432 --pg-username=postgres \
  --pg-password=postgres --pg-dbname=test > /dev/null 2>&1
assertExitCode 1 $?

echo "Test #$testNumber: invalid config file path"
docker run --rm "$IMAGE_NAME" /bin/device_reporter \
  --config=/nonexistent/config.yaml > /dev/null 2>&1
assertExitCode 1 $?

echo "Test #$testNumber: config is a directory, not a file"
docker run --rm "$IMAGE_NAME" /bin/device_reporter \
  --config=/tmp > /dev/null 2>&1
assertExitCode 1 $?

# ──────────────────────────────────────────
# Negative tests — migrator
# ──────────────────────────────────────────

log_section "Negative tests — migrator flags"

echo "Test #$testNumber: migrator missing --username"
docker run --rm "$IMAGE_NAME" /bin/migrate \
  --password=postgres --host=localhost --port=5432 --db=test > /dev/null 2>&1
assertExitCode 1 $?

echo "Test #$testNumber: migrator invalid type"
docker run --rm "$IMAGE_NAME" /bin/migrate \
  --type=invalid --username=postgres --password=postgres \
  --host=localhost --port=5432 --db=test > /dev/null 2>&1
assertExitCode 1 $?

# ──────────────────────────────────────────
# Happy path — обработка валидного файла
# ──────────────────────────────────────────

log_section "Happy path — valid TSV file"

reset_dirs
cleanup_db

cat > "$WATCH_DIR/data.tsv" <<'EOF'
n	mqtt	invid	unit_guid	msg_id	text	context	class	level	area	addr	block	type	bit	invert_bit
1		G-044322	01749246-95f6-57db-b7c3-2ae0e8be671f	cold7_Defrost_status	Разморозка		waiting	100	LOCAL	cold7_status.Defrost_status				
2		G-044322	01749246-95f6-57db-b7c3-2ae0e8be671f	cold7_VentSK_status	Вентилятор		working	100	LOCAL	cold7_status.VentSK_status				
3		G-044322	01749246-95f6-57db-b7c3-2ae0e8be671f	cold7_ComprSK_status	Компрессор		working	100	LOCAL	cold7_status.ComprSK_status				
EOF

echo "Running app for 3 seconds to process valid TSV..."
run_app 3

echo "Test #$testNumber: devices saved to DB"
assertDbCount "SELECT COUNT(*) FROM devices WHERE unit_guid = '01749246-95f6-57db-b7c3-2ae0e8be671f';" "3"

echo "Test #$testNumber: file status is 'done'"
assertDbValue "SELECT status FROM files WHERE name = 'data.tsv';" "done"

echo "Test #$testNumber: PDF report generated"
assertFileExists "$REPORTS_DIR/01749246-95f6-57db-b7c3-2ae0e8be671f.pdf"

echo "Test #$testNumber: file not reprocessed on second run"
run_app 3
assertDbCount "SELECT COUNT(*) FROM devices WHERE unit_guid = '01749246-95f6-57db-b7c3-2ae0e8be671f';" "3"

# ──────────────────────────────────────────
# Not happy path — невалидный файл
# ──────────────────────────────────────────

log_section "Not happy path — invalid TSV file"

reset_dirs
cleanup_db

cat > "$WATCH_DIR/bad.tsv" <<'EOF'
n	mqtt
1	mqtt
EOF

echo "Running app for 3 seconds to process invalid TSV..."
run_app 3

echo "Test #$testNumber: file status is 'error'"
assertDbValue "SELECT status FROM files WHERE name = 'bad.tsv';" "error"

echo "Test #$testNumber: error_message is not empty"
assertDbValue "SELECT error_message IS NOT NULL AND error_message != '' FROM files WHERE name = 'bad.tsv';" "t"

echo "Test #$testNumber: no devices saved for invalid file"
assertDbCount "SELECT COUNT(*) FROM devices;" "0"

echo "Test #$testNumber: no PDF generated for invalid file"
if [ -z "$(ls -A "$REPORTS_DIR")" ]; then
  echo -e "${GREEN}✓ Test #${testNumber} PASSED — no PDFs generated${NC}"
else
  echo -e "${RED}✗ Test #${testNumber} FAILED — unexpected PDFs found${NC}"
  failedTests=$((failedTests+1))
fi
testNumber=$((testNumber+1))

# ──────────────────────────────────────────
# API tests
# ──────────────────────────────────────────

log_section "API tests"

reset_dirs
cleanup_db

cat > "$WATCH_DIR/data.tsv" <<'EOF'
n	mqtt	invid	unit_guid	msg_id	text	context	class	level	area	addr	block	type	bit	invert_bit
1		G-044322	01749246-95f6-57db-b7c3-2ae0e8be671f	cold7_Defrost_status	Разморозка		waiting	100	LOCAL	cold7_status.Defrost_status				
2		G-044322	01749246-95f6-57db-b7c3-2ae0e8be671f	cold7_VentSK_status	Вентилятор		working	100	LOCAL	cold7_status.VentSK_status				
EOF

# Функции для выполнения HTTP-запросов через контейнер с curl (общая сеть с хостом)
function http_get_code() {
  local path="$1"
  docker run --rm --network host curlimages/curl:latest -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:8080${path}"
}

function http_get_body() {
  local path="$1"
  docker run --rm --network host curlimages/curl:latest -s "http://127.0.0.1:8080${path}"
}

# Ожидание запуска HTTP-сервера
wait_for_http() {
  local attempts=0
  echo "Waiting for HTTP server..."
  # Используем тот же контейнер curl для проверки
  until docker run --rm --network host curlimages/curl:latest -s -o /dev/null "http://127.0.0.1:8080/api/v1/devices/01749246-95f6-57db-b7c3-2ae0e8be671f" 2>/dev/null; do
    attempts=$((attempts+1))
    if [ $attempts -ge 30 ]; then
      echo -e "${RED}HTTP server did not start in time${NC}"
      # Покажем последние логи контейнера приложения (если доступны)
      if [ -n "$APP_PID" ]; then
        docker logs "$APP_PID" 2>/dev/null || echo "Container logs not available"
      fi
      return 1
    fi
    sleep 0.5
  done
  echo "HTTP server is ready"
}

echo "Running app with HTTP server..."
docker run --rm \
  --network host \
  -v "$TEST_CONFIG":/app/config.yaml \
  -v "$WATCH_DIR":/watch \
  -v "$REPORTS_DIR":/reports \
  "$IMAGE_NAME" /bin/device_reporter \
  --config=/app/config.yaml &

APP_PID=$!

# Ждём запуска HTTP-сервера
wait_for_http || exit 1

# Небольшая пауза для обработки файлов (опционально)
sleep 1

echo "Test #$testNumber: GET /api/v1/devices/{unit_guid} returns 200"
HTTP_CODE=$(http_get_code "/api/v1/devices/01749246-95f6-57db-b7c3-2ae0e8be671f?page=1&limit=10")
assertHttpCode 200 "$HTTP_CODE"

echo "Test #$testNumber: GET /api/v1/devices/{unit_guid} returns correct count"
# Получаем тело ответа и извлекаем total через jq (также в контейнере)
TOTAL=$(http_get_body "/api/v1/devices/01749246-95f6-57db-b7c3-2ae0e8be671f?page=1&limit=10" | docker run --rm -i ghcr.io/jqlang/jq:latest '.pagination.total')
if [ "$TOTAL" -eq 2 ]; then
  echo -e "${GREEN}✓ Test #${testNumber} PASSED — total = $TOTAL${NC}"
else
  echo -e "${RED}✗ Test #${testNumber} FAILED — expected total = 2, got $TOTAL${NC}"
  failedTests=$((failedTests+1))
fi
testNumber=$((testNumber+1))

echo "Test #$testNumber: GET /api/v1/devices/{unit_guid} with invalid page returns 400"
HTTP_CODE=$(http_get_code "/api/v1/devices/01749246-95f6-57db-b7c3-2ae0e8be671f?page=0")
assertHttpCode 400 "$HTTP_CODE"

echo "Test #$testNumber: GET /api/v1/devices/{unit_guid} with limit > 100 returns 400"
HTTP_CODE=$(http_get_code "/api/v1/devices/01749246-95f6-57db-b7c3-2ae0e8be671f?limit=101")
assertHttpCode 400 "$HTTP_CODE"

echo "Test #$testNumber: GET /api/v1/devices/{nonexistent_guid} returns empty data"
# Проверяем, что total = 0 (пустой результат)
TOTAL=$(http_get_body "/api/v1/devices/00000000-0000-0000-0000-000000000000?page=1&limit=10" | docker run --rm -i ghcr.io/jqlang/jq:latest '.pagination.total')
if [ "$TOTAL" -eq 0 ]; then
  echo -e "${GREEN}✓ Test #${testNumber} PASSED — total = $TOTAL${NC}"
else
  echo -e "${RED}✗ Test #${testNumber} FAILED — expected total = 0, got $TOTAL${NC}"
  failedTests=$((failedTests+1))
fi
testNumber=$((testNumber+1))

kill "$APP_PID" 2>/dev/null || true
wait "$APP_PID" 2>/dev/null || true
APP_PID=""

# ──────────────────────────────────────────
# Done
# ──────────────────────────────────────────

verifyAllTestsPassed
