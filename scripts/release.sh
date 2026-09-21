#!/usr/bin/env bash
# ==============================================================================
# scripts/release.sh - Automated release and version tagging script for debpub
# ==============================================================================
set -euo pipefail
IFS=$'\n\t'

# Color support for interactive UX
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m' # No Color

info() {
    echo -e "${BLUE}${BOLD}==>${NC} ${BOLD}$*${NC}"
}

success() {
    echo -e "${GREEN}${BOLD}==>${NC} ${GREEN}$*${NC}"
}

warn() {
    echo -e "${YELLOW}${BOLD}WARNING:${NC} $*"
}

error() {
    echo -e "${RED}${BOLD}ERROR:${NC} $*" >&2
}

# Resolve project root directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${ROOT_DIR}"

DRY_RUN=false
PUSH_MODE="" # "", "true", or "false"
VERSION_INPUT=""

# Parse arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --push)
            PUSH_MODE="true"
            shift
            ;;
        --no-push)
            PUSH_MODE="false"
            shift
            ;;
        -h|--help)
            cat <<EOF
Usage: $0 [options] <version>

Options:
  --dry-run       Validate checks and print diff without writing files or git tagging
  --push          Automatically push to origin without interactive confirmation prompt
  --no-push       Create local commit and tag only, do not push to origin
  -h, --help      Show this help message

Examples:
  $0 0.0.2
  $0 v0.0.2
  $0 --dry-run 0.0.2
EOF
            exit 0
            ;;
        *)
            if [[ -z "${VERSION_INPUT}" ]]; then
                VERSION_INPUT="$1"
            else
                error "Unexpected argument: $1"
                exit 1
            fi
            shift
            ;;
    esac
done

if [[ -z "${VERSION_INPUT}" ]]; then
    error "Version argument is required (e.g. $0 0.0.2 or make release VERSION=0.0.2)"
    exit 1
fi

# Normalize version: strip leading 'v' for raw semantic version
RAW_VERSION="${VERSION_INPUT#v}"
TAG_NAME="v${RAW_VERSION}"

# Validate semantic version format (e.g., 0.0.2, 1.2.3, 1.0.0-rc1)
if ! [[ "${RAW_VERSION}" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
    error "Invalid semantic version format: '${VERSION_INPUT}'. Expected format like 0.0.2 or v0.0.2"
    exit 1
fi

info "Preparing release for version: ${BOLD}${RAW_VERSION}${NC} (Git tag: ${BOLD}${TAG_NAME}${NC})"
if [[ "${DRY_RUN}" == "true" ]]; then
    warn "Running in DRY-RUN mode. No files or git state will be changed."
fi

# 1. Pre-flight checks
info "Running pre-flight checks..."

# Check current branch is main
CURRENT_BRANCH="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")"
if [[ "${CURRENT_BRANCH}" != "main" ]]; then
    error "Releases must be cut from the 'main' branch (currently on '${CURRENT_BRANCH}')."
    exit 1
fi

# Check working tree is clean
if [[ -n "$(git status --porcelain)" ]]; then
    error "Working tree has uncommitted or untracked changes. Stash or commit them before releasing."
    git status -s
    exit 1
fi

# Check if tag already exists locally
CURRENT_HEAD="$(git rev-parse HEAD 2>/dev/null || echo "")"
if git rev-parse "${TAG_NAME}" >/dev/null 2>&1; then
    TAG_COMMIT="$(git rev-parse "${TAG_NAME}^{commit}" 2>/dev/null || git rev-parse "${TAG_NAME}" 2>/dev/null)"
    if [[ "${TAG_COMMIT}" == "${CURRENT_HEAD}" ]]; then
        info "Git tag '${TAG_NAME}' already exists locally pointing to current HEAD (${CURRENT_HEAD:0:7})."
    else
        error "Git tag '${TAG_NAME}' already exists locally pointing to another commit (${TAG_COMMIT:0:7} != HEAD ${CURRENT_HEAD:0:7}). Delete or update it before releasing."
        exit 1
    fi
fi

# Check if tag already exists on origin (if remote is reachable)
REMOTE_TAGS=$(git ls-remote --tags origin "${TAG_NAME}" 2>/dev/null || true)
if echo "${REMOTE_TAGS}" | grep -q "${TAG_NAME}"; then
    error "Git tag '${TAG_NAME}' already exists on remote 'origin'."
    exit 1
fi

# 2. Run test suite before making any edits
info "Running test suite (make test)..."
if [[ "${DRY_RUN}" != "true" ]]; then
    make test
else
    echo "  [dry-run] Skipped make test execution."
fi

# 3. Update files with new version
info "Updating project version references to ${RAW_VERSION}..."

# Temporary helper to replace content safely on macOS and Linux without sed -i incompatibilities
safe_replace() {
    local pattern="$1"
    local replacement="$2"
    local file="$3"
    
    local tmp_file
    tmp_file="$(mktemp)"
    sed "s|${pattern}|${replacement}|g" "${file}" > "${tmp_file}"
    if [[ "${DRY_RUN}" == "true" ]]; then
        diff -u "${file}" "${tmp_file}" || true
        rm -f "${tmp_file}"
    else
        mv "${tmp_file}" "${file}"
    fi
}

# Target 1: internal/version/version.go
VERSION_GO="internal/version/version.go"
if [[ -f "${VERSION_GO}" ]]; then
    tmp_file="$(mktemp)"
    sed -e "s|Version = \"[^\"]*\"|Version = \"${RAW_VERSION}\"|g" \
        -e "s|Version=v[^\"]*\"|Version=v${RAW_VERSION}\"|g" \
        "${VERSION_GO}" > "${tmp_file}"
    if [[ "${DRY_RUN}" == "true" ]]; then
        echo "  [dry-run] Diff for ${VERSION_GO}:"
        diff -u "${VERSION_GO}" "${tmp_file}" || true
        rm -f "${tmp_file}"
    else
        mv "${tmp_file}" "${VERSION_GO}"
    fi
else
    error "File not found: ${VERSION_GO}"
    exit 1
fi

# Target 2: Makefile
MAKEFILE="Makefile"
if [[ -f "${MAKEFILE}" ]]; then
    safe_replace '^BASE_VERSION ?= .*$' "BASE_VERSION ?= ${RAW_VERSION}" "${MAKEFILE}"
else
    error "File not found: ${MAKEFILE}"
    exit 1
fi

# Target 3: Dockerfile
DOCKERFILE="Dockerfile"
if [[ -f "${DOCKERFILE}" ]]; then
    safe_replace '^ARG VERSION=.*$' "ARG VERSION=${RAW_VERSION}" "${DOCKERFILE}"
else
    error "File not found: ${DOCKERFILE}"
    exit 1
fi

# Target 4: README.md
README="README.md"
if [[ -f "${README}" ]]; then
    # Update download URL example
    safe_replace 'debpub_[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*' "debpub_${RAW_VERSION}" "${README}"
fi

if [[ "${DRY_RUN}" == "true" ]]; then
    success "Dry run completed successfully. No git modifications were made."
    exit 0
fi

# 4. Commit and Tag
git add "${VERSION_GO}" "${MAKEFILE}" "${DOCKERFILE}" "${README}"
if git diff --staged --quiet; then
    info "Version files already reflect version ${RAW_VERSION}. Using current commit $(git rev-parse --short HEAD)."
else
    info "Creating release commit..."
    git commit -m "chore: bump version to ${TAG_NAME}"
fi

# Create annotated tag if it does not already exist
if git rev-parse "${TAG_NAME}" >/dev/null 2>&1; then
    info "Annotated git tag ${TAG_NAME} already exists on HEAD."
else
    info "Creating annotated git tag: ${TAG_NAME}..."
    git tag -a "${TAG_NAME}" -m "Release ${TAG_NAME}"
fi

# 5. Display commit summary
echo ""
echo -e "${GREEN}${BOLD}Release commit and tag ready:${NC}"
git show --stat HEAD
echo ""

# 6. Push confirmation prompt
DO_PUSH=false

if [[ "${PUSH_MODE}" == "true" ]] || [[ "${PUSH:-}" == "true" ]]; then
    DO_PUSH=true
elif [[ "${PUSH_MODE}" == "false" ]]; then
    DO_PUSH=false
else
    # Interactive prompt (default)
    if [[ -t 0 ]]; then
        read -r -p "Proceed with push to origin (branch 'main' and tag '${TAG_NAME}')? [y/N]: " REPLY
        case "${REPLY}" in
            [yY]|[yY][eE][sS])
                DO_PUSH=true
                ;;
            *)
                DO_PUSH=false
                ;;
        esac
    else
        warn "Non-interactive shell detected and no push flag provided. Skipping remote push."
        DO_PUSH=false
    fi
fi

if [[ "${DO_PUSH}" == "true" ]]; then
    info "Pushing branch 'main' and tag '${TAG_NAME}' to origin..."
    git push origin main
    git push origin "${TAG_NAME}"
    success "Successfully pushed release ${TAG_NAME} to GitHub!"
    echo -e "GitHub Actions will now run the Release pipeline: https://github.com/fruitsforge/debpub/actions"
else
    warn "Push skipped. Local commit and tag are retained."
    echo -e "When ready, you can push manually by running:"
    echo -e "  ${BOLD}git push origin main && git push origin ${TAG_NAME}${NC}"
fi
