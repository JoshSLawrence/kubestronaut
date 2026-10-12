#!/usr/bin/env bash
set -euo pipefail

kubectl -n kube-system patch service kube-dns --type=merge \
  -p '{"spec":{"selector":{"k8s-app":"kube-dns"}}}'

tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT
kubectl -n kube-system get configmap coredns -o 'jsonpath={.data.Corefile}' >"$tmp"
# awk, not sed: the host may have BSD sed, which has no \n in replacements.
awk '
  { sub(/kubernetes cluster\.locl /, "kubernetes cluster.local ") }
  { print }
  /^[[:space:]]*errors$/ {
    print "    rewrite name legacy.example.internal legacy.apps.svc.cluster.local"
  }
' "$tmp" >"$tmp.new"
mv "$tmp.new" "$tmp"
kubectl -n kube-system create configmap coredns --from-file=Corefile="$tmp" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n kube-system rollout restart deployment/coredns
kubectl -n kube-system rollout status deployment/coredns --timeout=180s
