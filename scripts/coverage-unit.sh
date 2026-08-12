#!/usr/bin/env bash
# Run unit tests (skip envtest suites) and report filtered statement coverage.
# Excludes generated files from the coverage metric.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

COVERPROFILE="${COVERPROFILE:-coverage.out}"
FILTERED="${FILTERED_COVERPROFILE:-coverage-filtered.out}"
SKIP_PATTERN='TestHub|TestWorker|TestDeploy|TestManifest'

echo "Running unit tests with coverage..."
set +e
go test -count=1 -covermode=atomic \
  -coverprofile="${COVERPROFILE}" \
  -coverpkg=./api/...,./controllers/...,./pkg/...,./events/... \
  -skip "${SKIP_PATTERN}" \
  ./api/... ./controllers/... ./pkg/... ./events/... \
  2>&1 | tee /tmp/unit-test-run.log
TEST_EXIT=${PIPESTATUS[0]}
set -e

if [[ ! -f "${COVERPROFILE}" ]]; then
  echo "ERROR: coverage profile not produced (exit=${TEST_EXIT})"
  exit 1
fi

# Keep profile header; drop generated / deepcopy noise from metric.
awk '
  BEGIN { printed=0 }
  /^mode:/ {
    if (!printed) { print; printed=1 }
    next
  }
  /zz_generated\./ { next }
  /events_generated\.go/ { next }
  { print }
' "${COVERPROFILE}" > "${FILTERED}"

echo
echo "==== Filtered unit coverage (generated files excluded) ===="
go tool cover -func="${FILTERED}" | tee coverage-summary.txt | tail -n 5
echo
TOTAL_LINE=$(go tool cover -func="${FILTERED}" | grep total)
echo "==== TOTAL: ${TOTAL_LINE} ===="
# Also print failing packages from the log
echo
echo "==== Failed packages (if any) ===="
grep -E '^FAIL\s' /tmp/unit-test-run.log || echo "(none)"
exit 0
