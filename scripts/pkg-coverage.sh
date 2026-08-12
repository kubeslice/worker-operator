#!/usr/bin/env bash
set +e
pkgs=(
  ./controllers/slicegateway
  ./controllers/slice
  ./controllers/serviceexport
  ./controllers/serviceimport
  ./controllers
  ./pkg/hub/controllers
  ./pkg/hub/controllers/workerslicegwrecycler
  ./pkg/hub/hubclient
  ./pkg/hub/controllers/cluster
  ./pkg/manifest
  ./pkg/networkpolicy
  ./pkg/cluster
  ./pkg/webhook/pod
  ./pkg/namespace/controllers
  ./pkg/utils
  ./pkg/logger
  ./pkg/monitoring
  ./pkg/gwsidecar
  ./pkg/router
  ./pkg/netop
  ./pkg/gatewayedge
  ./pkg/slicegwrecycler
)
for pkg in "${pkgs[@]}"; do
  out=$(mktemp)
  go test -count=1 -covermode=atomic -coverprofile="$out" -skip 'TestHub|TestWorker|TestDeploy|TestManifest' "$pkg" > /tmp/t.log 2>&1
  st=$?
  pct=$(go tool cover -func="$out" 2>/dev/null | grep total | awk '{print $3}')
  echo "cov=${pct:-n/a} exit=${st} pkg=${pkg}"
  if [[ $st -ne 0 ]]; then
    grep -E '^--- FAIL:|Error Trace:|Error:|panic:' /tmp/t.log | head -8
    echo "---"
  fi
  rm -f "$out"
done
