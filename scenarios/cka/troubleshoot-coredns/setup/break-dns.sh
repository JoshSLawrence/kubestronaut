#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

# Two faults in two layers: the Corefile names the wrong zone for the
# kubernetes plugin, and the Service selects no CoreDNS pod.
tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT
kubectl -n kube-system get configmap coredns -o 'jsonpath={.data.Corefile}' >"$tmp"
grep -q 'kubernetes cluster\.local ' "$tmp"
sed -i.bak 's/kubernetes cluster\.local /kubernetes cluster.locl /' "$tmp"
rm -f "$tmp.bak"
kubectl -n kube-system create configmap coredns --from-file=Corefile="$tmp" \
  --dry-run=client -o yaml | kubectl apply -f -
kubectl -n kube-system patch service kube-dns --type=merge \
  -p '{"spec":{"selector":{"k8s-app":"kubedns"}}}'

# Do not wait for the reload plugin: restart so the first grade is stable.
kubectl -n kube-system rollout restart deployment/coredns
kubectl -n kube-system rollout status deployment/coredns --timeout=180s
for _ in $(seq 30); do
  eps=$(kubectl -n kube-system get endpointslices \
    -l kubernetes.io/service-name=kube-dns \
    -o 'jsonpath={.items[*].endpoints[*].addresses[*]}')
  [ -z "$eps" ] && exit 0
  sleep 2
done
echo "the kube-dns Service still has endpoints" >&2
exit 1
