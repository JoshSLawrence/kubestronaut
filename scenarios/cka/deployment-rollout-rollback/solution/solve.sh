#!/usr/bin/env bash
set -euo pipefail

kubectl -n web rollout undo deployment/frontend
kubectl -n web scale deployment/frontend --replicas=5
kubectl -n web patch deployment/frontend --type=merge -p \
  '{"spec":{"strategy":{"type":"RollingUpdate","rollingUpdate":{"maxSurge":1,"maxUnavailable":0}}}}'
kubectl -n web set resources deployment/frontend \
  --requests=cpu=50m,memory=64Mi
kubectl -n web rollout status deployment/frontend --timeout=120s
