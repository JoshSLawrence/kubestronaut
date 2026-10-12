#!/usr/bin/env bash
set -euo pipefail

# The Service selects labels no pod has, and targets a port nothing listens on.
kubectl -n store patch service checkout --type=merge \
  -p '{"spec":{"selector":{"app":"checkout"},"ports":[{"port":80,"targetPort":8080}]}}'

# The readiness probe checks port 8081, but the container listens on 8080.
kubectl -n store patch deployment checkout --type=json -p \
  '[{"op":"replace","path":"/spec/template/spec/containers/0/readinessProbe/httpGet/port","value":8080}]'
kubectl -n store rollout status deployment/checkout --timeout=120s
