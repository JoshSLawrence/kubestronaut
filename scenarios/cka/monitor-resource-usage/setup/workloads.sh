#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

bash setup/metrics-server.sh

# One of four pods burns CPU. Which one is random, so a guess cannot pass.
names=(batch-a batch-b batch-c batch-d)
busy=${names[$((RANDOM % 4))]}
ks_answer_put top-pod "$busy"
ks_log "creating the workloads"
kubectl create namespace metrics

pod() {
  local name=$1 app=$2 cmd=$3 limit=$4 mem=$5 node=${6:-}
  kubectl apply -f - <<YAML
apiVersion: v1
kind: Pod
metadata:
  name: $name
  namespace: metrics
  labels: {app: $app}
spec:
  terminationGracePeriodSeconds: 0
  ${node:+nodeName: $node}
  containers:
    - name: main
      image: busybox:1.38.0
      command: [sh, -c, "$cmd"]
      resources:
        limits: {cpu: "$limit", memory: $mem}
YAML
}

for n in "${names[@]}"; do
  if [ "$n" = "$busy" ]; then
    pod "$n" batch 'while :; do :; done' 300m 32Mi
  else
    pod "$n" batch 'sleep 3600' 100m 32Mi
  fi
done
# Busier than any batch pod, but not labelled app=batch: listing every pod in
# the namespace gives the wrong answer.
pod report-builder report 'while :; do :; done' 500m 32Mi
# Memory held on the worker, so that node and not the control plane is the
# heaviest: a file in a memory-backed volume counts as the pod's memory.
kubectl apply -f - <<YAML
apiVersion: v1
kind: Pod
metadata:
  name: memory-hog
  namespace: metrics
  labels: {app: hog}
spec:
  terminationGracePeriodSeconds: 0
  nodeName: $(ks_node_name node01)
  containers:
    - name: main
      image: busybox:1.38.0
      command: [sh, -c, "dd if=/dev/zero of=/hog/fill bs=1M count=1000 2>/dev/null; sleep 3600"]
      volumeMounts: [{name: hog, mountPath: /hog}]
      resources:
        limits: {cpu: 100m, memory: 1300Mi}
  volumes:
    - name: hog
      emptyDir: {medium: Memory, sizeLimit: 1200Mi}
YAML

kubectl -n metrics wait --for=condition=Ready pod --all --timeout=180s

# Wait for metrics to show the load, so the first grade is stable.
for _ in $(seq 90); do
  top=$(kubectl top pods -n metrics -l app=batch --no-headers --sort-by=cpu 2>/dev/null | head -n 1 | awk '{print $1}' || true)
  nodes=$(kubectl top nodes --no-headers 2>/dev/null | wc -l | tr -d ' ' || true)
  hog=$(kubectl top pod memory-hog -n metrics --no-headers 2>/dev/null | awk '{print $3}' || true)
  hog=${hog%Mi}
  case "$hog" in '' | *[!0-9]*) hog=0 ;; esac
  if [ "$top" = "$busy" ] && [ "$nodes" -ge 2 ] && [ "$hog" -ge 800 ]; then
    exit 0
  fi
  sleep 4
done
echo "metrics did not settle within 360s" >&2
exit 1
