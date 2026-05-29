#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG_DIR="$ROOT_DIR/.copilot-artifacts"
LOG_FILE="$LOG_DIR/playwright-tests.log"
HEADED=0
BACKGROUND=0
SLOWMO_MS="${PLAYWRIGHT_SLOWMO_MS:-0}"
BASE_URL="${PLAYWRIGHT_BASE_URL:-}"
RESET_STATE="${PLAYWRIGHT_RESET_STATE:-}"
LOGIN_EMAIL="${PLAYWRIGHT_LOGIN_EMAIL:-}"
LOGIN_PASSWORD="${PLAYWRIGHT_LOGIN_PASSWORD:-}"
ADMIN_LOGIN_EMAIL="${PLAYWRIGHT_ADMIN_LOGIN_EMAIL:-}"
ADMIN_LOGIN_PASSWORD="${PLAYWRIGHT_ADMIN_LOGIN_PASSWORD:-}"
MANAGER_LOGIN_EMAIL="${PLAYWRIGHT_MANAGER_LOGIN_EMAIL:-}"
MANAGER_LOGIN_PASSWORD="${PLAYWRIGHT_MANAGER_LOGIN_PASSWORD:-}"
EMPLOYEE_LOGIN_EMAIL="${PLAYWRIGHT_EMPLOYEE_LOGIN_EMAIL:-}"
EMPLOYEE_LOGIN_PASSWORD="${PLAYWRIGHT_EMPLOYEE_LOGIN_PASSWORD:-}"

usage() {
  cat <<'EOF'
Usage: scripts/run-playwright-tests.sh [options] [-- unittest args...]

Options:
  --headed            Run Playwright with a visible browser window
  --background        Run the test command under nohup and return immediately
  --slowmo-ms <ms>    Add Playwright slow motion delay in milliseconds
  --base-url <url>    Target an already running site instead of fixture test server
  --no-reset          Skip POST /__reset before each test (recommended with --base-url)
  --login-email <v>   Login email used by live-mode auth path checks
  --login-password <v> Login password used by live-mode auth path checks
  --admin-login-email <v> Role-session admin email for duty audit
  --admin-login-password <v> Role-session admin password for duty audit
  --manager-login-email <v> Role-session manager email for duty audit
  --manager-login-password <v> Role-session manager password for duty audit
  --employee-login-email <v> Role-session employee email for duty audit
  --employee-login-password <v> Role-session employee password for duty audit
  --log-file <path>   Write background output to a specific log file
  --help              Show this help text

Examples:
  scripts/run-playwright-tests.sh --headed --slowmo-ms 250
  scripts/run-playwright-tests.sh --background
  scripts/run-playwright-tests.sh --headed --base-url http://127.0.0.1:8080 --no-reset --login-email maria.garcia@usda.gov --login-password password123 -- -k visual_site_sweep
  scripts/run-playwright-tests.sh --headed -- --k test_social_surfaces
EOF
}

ARGS=()
while (($#)); do
  case "$1" in
    --headed)
      HEADED=1
      shift
      ;;
    --background)
      BACKGROUND=1
      shift
      ;;
    --slowmo-ms)
      SLOWMO_MS="${2:-}"
      shift 2
      ;;
    --base-url)
      BASE_URL="${2:-}"
      shift 2
      ;;
    --no-reset)
      RESET_STATE=0
      shift
      ;;
    --login-email)
      LOGIN_EMAIL="${2:-}"
      shift 2
      ;;
    --login-password)
      LOGIN_PASSWORD="${2:-}"
      shift 2
      ;;
    --admin-login-email)
      ADMIN_LOGIN_EMAIL="${2:-}"
      shift 2
      ;;
    --admin-login-password)
      ADMIN_LOGIN_PASSWORD="${2:-}"
      shift 2
      ;;
    --manager-login-email)
      MANAGER_LOGIN_EMAIL="${2:-}"
      shift 2
      ;;
    --manager-login-password)
      MANAGER_LOGIN_PASSWORD="${2:-}"
      shift 2
      ;;
    --employee-login-email)
      EMPLOYEE_LOGIN_EMAIL="${2:-}"
      shift 2
      ;;
    --employee-login-password)
      EMPLOYEE_LOGIN_PASSWORD="${2:-}"
      shift 2
      ;;
    --log-file)
      LOG_FILE="${2:-}"
      shift 2
      ;;
    --help)
      usage
      exit 0
      ;;
    --)
      shift
      while (($#)); do
        ARGS+=("$1")
        shift
      done
      ;;
    *)
      ARGS+=("$1")
      shift
      ;;
  esac
done

HAS_VERBOSE=0
for arg in "${ARGS[@]}"; do
  if [[ "$arg" == "-v" || "$arg" == "--verbose" ]]; then
    HAS_VERBOSE=1
    break
  fi
done

mkdir -p "$LOG_DIR"

CMD=(
  uv run
  python -m unittest discover
  -s tests
  -p 'test_*.py'
)

if ((${#ARGS[@]})); then
  CMD+=("${ARGS[@]}")
fi

if ((HAS_VERBOSE == 0)); then
  CMD+=("-v")
fi

export PLAYWRIGHT_HEADED="$HEADED"
export PLAYWRIGHT_SLOWMO_MS="$SLOWMO_MS"
if [[ -n "$BASE_URL" ]]; then
  export PLAYWRIGHT_BASE_URL="$BASE_URL"
fi
if [[ -n "$RESET_STATE" ]]; then
  export PLAYWRIGHT_RESET_STATE="$RESET_STATE"
fi
if [[ -n "$LOGIN_EMAIL" ]]; then
  export PLAYWRIGHT_LOGIN_EMAIL="$LOGIN_EMAIL"
fi
if [[ -n "$LOGIN_PASSWORD" ]]; then
  export PLAYWRIGHT_LOGIN_PASSWORD="$LOGIN_PASSWORD"
fi
if [[ -n "$ADMIN_LOGIN_EMAIL" ]]; then
  export PLAYWRIGHT_ADMIN_LOGIN_EMAIL="$ADMIN_LOGIN_EMAIL"
fi
if [[ -n "$ADMIN_LOGIN_PASSWORD" ]]; then
  export PLAYWRIGHT_ADMIN_LOGIN_PASSWORD="$ADMIN_LOGIN_PASSWORD"
fi
if [[ -n "$MANAGER_LOGIN_EMAIL" ]]; then
  export PLAYWRIGHT_MANAGER_LOGIN_EMAIL="$MANAGER_LOGIN_EMAIL"
fi
if [[ -n "$MANAGER_LOGIN_PASSWORD" ]]; then
  export PLAYWRIGHT_MANAGER_LOGIN_PASSWORD="$MANAGER_LOGIN_PASSWORD"
fi
if [[ -n "$EMPLOYEE_LOGIN_EMAIL" ]]; then
  export PLAYWRIGHT_EMPLOYEE_LOGIN_EMAIL="$EMPLOYEE_LOGIN_EMAIL"
fi
if [[ -n "$EMPLOYEE_LOGIN_PASSWORD" ]]; then
  export PLAYWRIGHT_EMPLOYEE_LOGIN_PASSWORD="$EMPLOYEE_LOGIN_PASSWORD"
fi

cd "$ROOT_DIR"

if ((BACKGROUND)); then
  nohup "${CMD[@]}" >"$LOG_FILE" 2>&1 &
  PID=$!
  echo "Started Playwright tests in background."
  echo "PID: $PID"
  echo "Log: $LOG_FILE"
  exit 0
fi

python3 scripts/playwright_progress_runner.py --repo-root "$ROOT_DIR" -- "${CMD[@]}"
