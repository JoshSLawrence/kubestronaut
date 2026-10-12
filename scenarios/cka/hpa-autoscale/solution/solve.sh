#!/usr/bin/env bash
set -euo pipefail

# Utilisation is a percentage of the CPU request, so the pods need one.
kubectl -n autoscale set resources deployment/php-apache --requests=cpu=100m

kubectl apply -f - <<'YAML'
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: php-apache
  namespace: autoscale
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: php-apache
  minReplicas: 1
  maxReplicas: 4
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 50
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 30
YAML
kubectl -n autoscale rollout status deployment/php-apache --timeout=120s
