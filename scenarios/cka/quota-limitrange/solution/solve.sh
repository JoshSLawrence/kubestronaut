#!/usr/bin/env bash
set -euo pipefail

kubectl apply -f - <<'YAML'
apiVersion: v1
kind: LimitRange
metadata:
  name: container-defaults
  namespace: team-a
spec:
  limits:
    - type: Container
      defaultRequest: {cpu: 100m, memory: 64Mi}
      default: {cpu: 200m, memory: 128Mi}
      max: {cpu: 500m}
YAML

kubectl -n team-a create quota team-a-quota \
  --hard=pods=5,requests.cpu=1,limits.memory=1Gi

# Defaults are applied when a pod is created, so replace the running ones.
kubectl -n team-a rollout restart deployment/web
kubectl -n team-a rollout status deployment/web --timeout=120s
