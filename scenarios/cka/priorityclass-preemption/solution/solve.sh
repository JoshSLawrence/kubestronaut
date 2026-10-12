#!/usr/bin/env bash
set -euo pipefail

kubectl apply -f - <<'YAML'
apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
metadata:
  name: high-priority
value: 749999
globalDefault: false
description: Just below tier-gold.
YAML

kubectl -n prod patch deployment api --type=strategic -p \
  '{"spec":{"template":{"spec":{"priorityClassName":"high-priority"}}}}'
kubectl -n prod rollout status deployment/api --timeout=180s
