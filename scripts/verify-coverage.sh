#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# DevClean v2.3 — Per-Package Statement Coverage Auditor (VAL-02, D-09, D-10)
# Enforces strict >= 85.0% statement coverage on all 11 altered or new packages.
# ==============================================================================

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

MIN_COVERAGE=85.0
FAILURES=0

TARGET_PACKAGES=(
  "internal/cleaner/macos"
  "internal/cleaner/browsers"
  "internal/cleaner/leftovers"
  "internal/cleaner/node"
  "internal/cleaner/shell"
  "internal/cleaner/xcode"
  "internal/cleaner/ai"
  "internal/cleaner/docker"
  "internal/deleter"
  "internal/engine"
  "internal/domain"
)

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m'

echo -e "${BLUE}${BOLD}=== Auditing DevClean v2.3 Package Coverage (Floor: ${MIN_COVERAGE}%) ===${NC}"
printf "%-35s %-12s %s\n" "Package" "Coverage" "Status"
printf "%-35s %-12s %s\n" "-----------------------------------" "------------" "------"

for pkg in "${TARGET_PACKAGES[@]}"; do
  output=$(go test -cover "./$pkg" 2>&1) || {
    echo -e "${RED}❌ [FAIL]${NC} $pkg: Compilation / test run failed"
    echo "$output"
    FAILURES=$((FAILURES + 1))
    continue
  }

  cov_str=$(echo "$output" | grep -o 'coverage: [0-9.]*%' | awk '{print $2}' | tr -d '%' || true)
  if [ -z "$cov_str" ]; then
    echo -e "${RED}❌ [FAIL]${NC} $pkg: Could not extract coverage statement"
    FAILURES=$((FAILURES + 1))
    continue
  fi

  is_passing=$(awk "BEGIN {print ($cov_str >= $MIN_COVERAGE) ? 1 : 0}")
  if [ "$is_passing" -eq 1 ]; then
    printf "%-35s %-12s ${GREEN}✔ PASS${NC}\n" "$pkg" "${cov_str}%"
  else
    printf "%-35s %-12s ${RED}✖ FAIL (< ${MIN_COVERAGE}%)${NC}\n" "$pkg" "${cov_str}%"
    FAILURES=$((FAILURES + 1))
  fi
done

echo ""
if [ "$FAILURES" -gt 0 ]; then
  echo -e "${RED}${BOLD}Audit failed: $FAILURES package(s) fell below the required ${MIN_COVERAGE}% statement coverage floor.${NC}"
  exit 1
fi

echo -e "${GREEN}${BOLD}Audit successful: All ${#TARGET_PACKAGES[@]} v2.3 packages meet or exceed the ${MIN_COVERAGE}% coverage floor.${NC}"
exit 0
