#!/usr/bin/env bash
set -euo pipefail

kubectl -n ml patch deployment trainer --type=strategic -p '
spec:
  template:
    spec:
      tolerations:
        - key: dedicated
          operator: Equal
          value: gpu
          effect: NoSchedule
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                  - key: accelerator
                    operator: In
                    values: [gpu]
'
kubectl -n ml rollout status deployment/trainer --timeout=120s
