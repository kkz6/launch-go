{{/* Common helper functions used across all scripts */}}

{{define "helpers"}}
# ============================================================================
# Helper Functions
# ============================================================================

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print colored output
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# HTTP POST helper (silent)
httpPostSilently() {
    local url="$1"
    local data="${2:-}"

    if command -v curl &> /dev/null; then
        if [ -n "$data" ]; then
            curl -s -X POST -H "Content-Type: application/json" -d "$data" "$url" > /dev/null 2>&1 || true
        else
            curl -s -X POST "$url" > /dev/null 2>&1 || true
        fi
    elif command -v wget &> /dev/null; then
        if [ -n "$data" ]; then
            wget -q --post-data="$data" --header="Content-Type: application/json" -O /dev/null "$url" 2>&1 || true
        else
            wget -q --post-data="" -O /dev/null "$url" 2>&1 || true
        fi
    fi
}

# Wait for apt lock to be released
wait_for_apt() {
    while fuser /var/lib/dpkg/lock >/dev/null 2>&1 || \
          fuser /var/lib/apt/lists/lock >/dev/null 2>&1 || \
          fuser /var/lib/dpkg/lock-frontend >/dev/null 2>&1; do
        print_info "Waiting for apt lock..."
        sleep 5
    done
}

# Retry command with exponential backoff
retry() {
    local max_attempts="$1"
    local delay="$2"
    shift 2
    local attempt=1

    while true; do
        "$@" && break || {
            if [[ $attempt -lt $max_attempts ]]; then
                print_warning "Command failed. Attempt $attempt/$max_attempts. Retrying in ${delay}s..."
                sleep $delay
                attempt=$((attempt + 1))
                delay=$((delay * 2))
            else
                print_error "Command failed after $attempt attempts"
                return 1
            fi
        }
    done
}

# Check if command exists
command_exists() {
    command -v "$1" &> /dev/null
}

# Create directory if not exists
ensure_dir() {
    [ -d "$1" ] || mkdir -p "$1"
}

# Backup file before modifying
backup_file() {
    local file="$1"
    if [ -f "$file" ]; then
        cp "$file" "${file}.backup.$(date +%Y%m%d%H%M%S)"
    fi
}
{{end}}
