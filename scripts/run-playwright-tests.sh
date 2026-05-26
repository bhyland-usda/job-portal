#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG_DIR="$ROOT_DIR/.copilot-artifacts"
LOG_FILE="$LOG_DIR/playwright-tests.log"
HEADED=0
BACKGROUND=0
SLOWMO_MS="${PLAYWRIGHT_SLOWMO_MS:-0}"

usage() {
  cat <<'EOF'
Usage: scripts/run-playwright-tests.sh [options] [-- unittest args...]

Options:
  --headed            Run Playwright with a visible browser window
  --background        Run the test command under nohup and return immediately
  --slowmo-ms <ms>    Add Playwright slow motion delay in milliseconds
  --log-file <path>   Write background output to a specific log file
  --help              Show this help text

Examples:
  scripts/run-playwright-tests.sh --headed --slowmo-ms 250
  scripts/run-playwright-tests.sh --background
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

export PLAYWRIGHT_HEADED="$HEADED"
export PLAYWRIGHT_SLOWMO_MS="$SLOWMO_MS"

cd "$ROOT_DIR"

if ((BACKGROUND)); then
  nohup "${CMD[@]}" >"$LOG_FILE" 2>&1 &
  PID=$!
  echo "Started Playwright tests in background."
  echo "PID: $PID"
  echo "Log: $LOG_FILE"
  exit 0
fi

"${CMD[@]}"
