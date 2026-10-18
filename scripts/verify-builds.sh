#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# DevClean v2.3 — Zero-CGO Static Cross-Compilation Matrix (VAL-02, D-11)
# Validates static compilation across Darwin and Linux targets with CGO_ENABLED=0.
# ==============================================================================

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

TARGETS=(
  "darwin/arm64"
  "darwin/amd64"
  "linux/amd64"
  "linux/arm64"
)

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m'

echo -e "${BLUE}${BOLD}=== Verifying Static Zero-CGO Cross-Compilation Matrix ===${NC}"
FAILURES=0

for target in "${TARGETS[@]}"; do
  os="${target%/*}"
  arch="${target#*/}"
  echo -n "Building static binary for $os/$arch (CGO_ENABLED=0)... "
  if CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -o /dev/null . 2>&1; then
    echo -e "${GREEN}✔ SUCCESS${NC}"
  else
    echo -e "${RED}✖ FAILED${NC}"
    FAILURES=$((FAILURES + 1))
  fi
done

echo ""
if [ "$FAILURES" -gt 0 ]; then
  echo -e "${RED}${BOLD}Build matrix verification failed: $FAILURES target(s) failed compilation.${NC}"
  exit 1
fi

echo -e "${GREEN}${BOLD}Build matrix successful: All ${#TARGETS[@]} static cross-targets compiled cleanly without CGO.${NC}"
exit 0
