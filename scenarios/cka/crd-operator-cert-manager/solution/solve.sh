#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

kubectl apply --server-side -f - < <(ks_node controlplane cat /opt/operators/cert-manager.yaml)
kubectl -n cert-manager patch deployment cert-manager --type json -p \
  '[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--enable-certificate-owner-ref"}]'
for d in cert-manager cert-manager-webhook cert-manager-cainjector; do
  kubectl -n cert-manager rollout status "deployment/$d" --timeout=300s
done

kubectl get crd -o name | sed 's#.*/##' | grep 'cert-manager\.io$' |
  ks_node controlplane sh -c 'cat > /root/cert-manager-crds.txt'
kubectl explain certificate.spec.subject |
  ks_node controlplane sh -c 'cat > /root/subject.txt'

manifest=$(
  cat <<'YAML'
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: selfsigned
spec:
  selfSigned: {}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: web-cert
  namespace: shop
spec:
  secretName: web-tls
  dnsNames: [web.shop.svc]
  issuerRef:
    kind: ClusterIssuer
    name: selfsigned
YAML
)
# The webhook may still be starting: retry until it admits the objects.
for _ in $(seq 30); do
  if printf '%s\n' "$manifest" | kubectl apply -f -; then
    break
  fi
  sleep 5
done
kubectl -n shop wait --for=condition=Ready certificate/web-cert --timeout=120s
