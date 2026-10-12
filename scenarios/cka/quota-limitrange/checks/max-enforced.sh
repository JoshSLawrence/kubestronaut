#!/usr/bin/env bash
set -euo pipefail

read -r -d '' pod <<'YAML' || true
apiVersion: v1
kind: Pod
metadata:
  name: ks-limit-probe
spec:
  containers:
    - name: probe
      image: registry.k8s.io/e2e-test-images/agnhost:2.61
      args: [pause]
      resources:
        requests: {cpu: 50m, memory: 32Mi}
        limits: {cpu: "1", memory: 64Mi}
YAML

# A server-side dry run goes through admission, so it shows what the
# LimitRange really enforces without leaving a pod behind.
set +e
out=$(kubectl -n team-a apply --dry-run=server -f - 2>&1 <<<"$pod")
rc=$?
set -e
if [ "$rc" -eq 0 ]; then
  echo "a container with a 1 CPU limit was accepted" >&2
  exit 1
fi
if ! grep -qi 'maximum cpu usage' <<<"$out"; then
  echo "the 1 CPU probe was rejected, but not for its CPU limit" >&2
  exit 1
fi
