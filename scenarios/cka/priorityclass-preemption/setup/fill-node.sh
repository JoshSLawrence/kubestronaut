#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

# Quantities are "250m" or whole cores, which is all the node and the system
# pods ever use.
millis() {
  case "$1" in
    *m) echo "${1%m}" ;;
    *) echo $(($1 * 1000)) ;;
  esac
}

node=$(ks_node_name controlplane)
alloc=$(millis "$(kubectl get node "$node" -o 'jsonpath={.status.allocatable.cpu}')")
used=0
for q in $(kubectl get pods -A --field-selector "spec.nodeName=$node" \
  -o 'jsonpath={range .items[*]}{range .spec.containers[*]}{.resources.requests.cpu}{"\n"}{end}{end}'); do
  used=$((used + $(millis "$q")))
done
free=$((alloc - used))

# Leave 300m free, so a 500m pod does not fit until one batch pod goes, and
# make each batch pod big enough that removing one makes room.
per=$(((free - 300) / 4))
if [ "$per" -lt 200 ]; then
  echo "the node has too little CPU to build this scenario (${free}m free)" >&2
  exit 1
fi
ks_log "allocatable ${alloc}m, requested ${used}m, filler pods ${per}m each"

kubectl apply -f - <<YAML
apiVersion: apps/v1
kind: Deployment
metadata:
  name: filler
  namespace: batch
spec:
  replicas: 4
  selector:
    matchLabels:
      app: filler
  template:
    metadata:
      labels:
        app: filler
    spec:
      priorityClassName: tier-bronze
      containers:
        - name: filler
          image: registry.k8s.io/e2e-test-images/agnhost:2.61
          args: [pause]
          resources:
            requests:
              cpu: ${per}m
YAML
kubectl -n batch rollout status deployment/filler --timeout=120s

kubectl apply -f - <<'YAML'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  namespace: prod
spec:
  replicas: 1
  selector:
    matchLabels:
      app: api
  template:
    metadata:
      labels:
        app: api
    spec:
      containers:
        - name: api
          image: registry.k8s.io/e2e-test-images/agnhost:2.61
          args: [pause]
          resources:
            requests:
              cpu: 500m
YAML
# The pod must be visibly unschedulable, so the first grade is stable.
for _ in $(seq 30); do
  reason=$(kubectl -n prod get pods -l app=api \
    -o 'jsonpath={.items[0].status.conditions[?(@.type=="PodScheduled")].reason}')
  [ "$reason" = "Unschedulable" ] && exit 0
  sleep 2
done
echo "the api pod was scheduled; the node has more room than expected" >&2
exit 1
