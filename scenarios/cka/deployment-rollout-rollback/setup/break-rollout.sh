#!/usr/bin/env bash
set -euo pipefail

# A tag that does not exist keeps the second revision from ever becoming
# ready, so the rollout stalls while revision 1 keeps serving.
kubectl -n web set image deployment/frontend \
  frontend=registry.k8s.io/e2e-test-images/agnhost:2.61-missing

# Return only once the new pod is visibly stuck, so the first grade is stable.
for _ in $(seq 60); do
  reason=$(kubectl -n web get pods -l app=frontend \
    -o 'jsonpath={.items[*].status.containerStatuses[0].state.waiting.reason}')
  case "$reason" in
    *ErrImagePull* | *ImagePullBackOff*) exit 0 ;;
  esac
  sleep 2
done
echo "the new pod never failed to pull its image" >&2
exit 1
