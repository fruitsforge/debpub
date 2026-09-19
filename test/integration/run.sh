#!/bin/bash
set -eo pipefail

# ==============================================================================
# debpub Integration Test Runner
# Tests MinIO (S3 conditional writes & locks) and SFTP (atomic POSIX operations)
# ==============================================================================

# ANSI Colors for UX
COLOR_RESET="\033[0m"
COLOR_BOLD="\033[1m"
COLOR_CYAN="\033[36m"
COLOR_GREEN="\033[32m"
COLOR_YELLOW="\033[33m"
COLOR_RED="\033[31m"
COLOR_BLUE="\033[34m"

log_info() {
    echo -e "${COLOR_CYAN}[INFO]${COLOR_RESET} $1"
}

log_step() {
    echo -e "\n${COLOR_BOLD}${COLOR_BLUE}==>${COLOR_RESET} ${COLOR_BOLD}$1${COLOR_RESET}"
}

log_pass() {
    echo -e "  ${COLOR_GREEN}✔ [PASS]${COLOR_RESET} $1"
}

log_fail() {
    echo -e "  ${COLOR_RED}✖ [FAIL]${COLOR_RESET} $1"
}

log_warn() {
    echo -e "  ${COLOR_YELLOW}⚠ [WARN]${COLOR_RESET} $1"
}

START_TIME=$(date +%s)
FAILED_TESTS=0

echo -e "${COLOR_BOLD}${COLOR_CYAN}======================================================${COLOR_RESET}"
echo -e "${COLOR_BOLD}${COLOR_CYAN}         debpub End-to-End Integration Suite         ${COLOR_RESET}"
echo -e "${COLOR_BOLD}${COLOR_CYAN}======================================================${COLOR_RESET}"

# ------------------------------------------------------------------------------
# 1. Dependency Probing & Health Checks
# ------------------------------------------------------------------------------
log_step "Probing Service Endpoints (MinIO S3 & SFTP)"

# S3 Healthcheck
S3_ENDPOINT=${S3_ENDPOINT:-"http://minio:9000"}
log_info "Connecting to MinIO at ${S3_ENDPOINT}..."
MAX_RETRIES=15
RETRY=0
until curl -s -f "${S3_ENDPOINT}/minio/health/live" >/dev/null 2>&1; do
    RETRY=$((RETRY+1))
    if [ $RETRY -ge $MAX_RETRIES ]; then
        log_fail "MinIO failed to become ready within deadline"
        exit 1
    fi
    sleep 1
done
log_pass "MinIO service is healthy (${S3_ENDPOINT})"

# SFTP Healthcheck
SFTP_HOST=${SFTP_HOST:-"sftp"}
SFTP_PORT=${SFTP_PORT:-"22"}
log_info "Connecting to SFTP service at ${SFTP_HOST}:${SFTP_PORT}..."
RETRY=0
until nc -z "${SFTP_HOST}" "${SFTP_PORT}" >/dev/null 2>&1 || (echo >/dev/tcp/"${SFTP_HOST}"/"${SFTP_PORT}") >/dev/null 2>&1; do
    RETRY=$((RETRY+1))
    if [ $RETRY -ge $MAX_RETRIES ]; then
        log_fail "SFTP server failed to respond within deadline"
        exit 1
    fi
    sleep 1
done
log_pass "SFTP service is listening on ${SFTP_HOST}:${SFTP_PORT}"

# Ensure MinIO bucket exists
S3_BUCKET=${S3_BUCKET:-"debian-test"}
log_info "Verifying S3 target bucket '${S3_BUCKET}'..."
# Use debpub or python/curl or test to initialize bucket if needed
# MinIO allows creating bucket via simple S3 PutBucket call, or our Go test creates it.

# ------------------------------------------------------------------------------
# 2. Execute Native Go Integration Test Suite
# ------------------------------------------------------------------------------
log_step "Stage 1: Running Native Go Integration Tests (-tags=integration)"

export S3_ENDPOINT
export S3_BUCKET
export AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID:-"minioadmin"}
export AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY:-"minioadminpassword"}
export AWS_REGION=${AWS_REGION:-"us-east-1"}
export SFTP_HOST
export SFTP_PORT
export SFTP_USER=${SFTP_USER:-"testuser"}
export SFTP_PASSWORD=${SFTP_PASSWORD:-"testpassword"}

if go test -v -tags=integration ./test/integration/...; then
    log_pass "Go integration tests passed (MinIO S3 conditional locking + SFTP atomic locking)"
else
    log_fail "Go integration tests failed"
    FAILED_TESTS=$((FAILED_TESTS+1))
fi

# ------------------------------------------------------------------------------
# 3. Execute End-to-End CLI Publishing Test with debpub binary
# ------------------------------------------------------------------------------
log_step "Stage 2: Running End-to-End CLI Publishing Tests"

TMP_DIR=$(mktemp -d)
trap 'rm -rf "${TMP_DIR}"' EXIT

# Generate a sample deb package using dpkg-deb
mkdir -p "${TMP_DIR}/pkg/DEBIAN"
mkdir -p "${TMP_DIR}/pkg/usr/bin"
cat <<EOF > "${TMP_DIR}/pkg/DEBIAN/control"
Package: testcli
Version: 1.0.0
Architecture: amd64
Maintainer: Debpub Team <team@debpub.local>
Description: Test package created by integration test runner
EOF

cat <<EOF > "${TMP_DIR}/pkg/usr/bin/testcli"
#!/bin/sh
echo "hello from debpub test package"
EOF
chmod 755 "${TMP_DIR}/pkg/usr/bin/testcli"

dpkg-deb --build "${TMP_DIR}/pkg" "${TMP_DIR}/testcli_1.0.0_amd64.deb" >/dev/null 2>&1
log_info "Built test deb package: testcli_1.0.0_amd64.deb"

# 3A. Test debpub CLI with MinIO S3
log_info "Executing CLI publish to MinIO (s3://${S3_BUCKET}/cli-repo)..."
if /usr/local/bin/debpub publish \
    --storage s3 \
    -b "${S3_BUCKET}" \
    --prefix cli-repo \
    --s3-endpoint "${S3_ENDPOINT}" \
    --s3-force-path-style \
    -c bookworm \
    -m main \
    "${TMP_DIR}/testcli_1.0.0_amd64.deb"; then
    log_pass "CLI publish to MinIO S3 succeeded"
else
    log_fail "CLI publish to MinIO S3 failed"
    FAILED_TESTS=$((FAILED_TESTS+1))
fi

# 3B. Test debpub CLI with SFTP
log_info "Executing CLI publish to SFTP (sftp://${SFTP_HOST}:${SFTP_PORT}/home/testuser/debian/cli-repo)..."
if /usr/local/bin/debpub publish \
    --storage sftp \
    --sftp-host "${SFTP_HOST}" \
    --sftp-port "${SFTP_PORT}" \
    --sftp-user "${SFTP_USER}" \
    --sftp-password "${SFTP_PASSWORD}" \
    --prefix "/home/testuser/debian/cli-repo" \
    -c bookworm \
    -m main \
    "${TMP_DIR}/testcli_1.0.0_amd64.deb"; then
    log_pass "CLI publish to SFTP succeeded"
else
    log_fail "CLI publish to SFTP failed"
    FAILED_TESTS=$((FAILED_TESTS+1))
fi

# ------------------------------------------------------------------------------
# 4. Final Executive Summary
# ------------------------------------------------------------------------------
END_TIME=$(date +%s)
ELAPSED=$((END_TIME - START_TIME))

echo -e "\n${COLOR_BOLD}${COLOR_CYAN}======================================================${COLOR_RESET}"
echo -e "${COLOR_BOLD}${COLOR_CYAN}               Integration Test Summary               ${COLOR_RESET}"
echo -e "${COLOR_BOLD}${COLOR_CYAN}======================================================${COLOR_RESET}"
echo -e "Total Time: ${ELAPSED}s"

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "Result:     ${COLOR_GREEN}${COLOR_BOLD}ALL INTEGRATION TESTS PASSED (MinIO S3 + SFTP)${COLOR_RESET}"
    echo -e "${COLOR_BOLD}${COLOR_CYAN}======================================================${COLOR_RESET}\n"
    exit 0
else
    echo -e "Result:     ${COLOR_RED}${COLOR_BOLD}SOME TESTS FAILED (${FAILED_TESTS} failures)${COLOR_RESET}"
    echo -e "${COLOR_BOLD}${COLOR_CYAN}======================================================${COLOR_RESET}\n"
    exit 1
fi
