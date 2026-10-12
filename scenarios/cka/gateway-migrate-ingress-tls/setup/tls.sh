#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

# The certificate is made on controlplane, which always has openssl; the host
# may only have LibreSSL.
ks_log "creating the TLS Secret and the Ingress"
ks_node controlplane bash -s <<'SH'
set -euo pipefail
dir=$(mktemp -d)
openssl req -x509 -newkey rsa:2048 -nodes -days 365 \
  -subj /CN=web.example.local -addext subjectAltName=DNS:web.example.local \
  -keyout "$dir/tls.key" -out "$dir/tls.crt" 2>/dev/null
kubectl -n web create secret tls web-tls --cert="$dir/tls.crt" --key="$dir/tls.key"
rm -rf "$dir"
SH

crt=$(kubectl -n web get secret web-tls -o 'jsonpath={.data.tls\.crt}')
fp=$(printf '%s' "$crt" | ks_node controlplane bash -c 'base64 -d | openssl x509 -noout -fingerprint -sha256')
ks_answer_put web-cert "$fp"

kubectl apply -f - <<'YAML'
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: web
  namespace: web
spec:
  ingressClassName: traefik
  tls:
    - hosts: [web.example.local]
      secretName: web-tls
  rules:
    - host: web.example.local
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: web
                port:
                  number: 80
YAML

# The solution is only fair once the Ingress really serves, so wait for it.
ips=$(kubectl get node "$KS_NODE_CONTROLPLANE" \
  -o 'jsonpath={.status.addresses[?(@.type=="InternalIP")].address}')
ip=${ips%% *}
for _ in $(seq 30); do
  body=$(ks_node controlplane curl -sk --max-time 3 \
    --resolve "web.example.local:30443:$ip" https://web.example.local:30443/ || true)
  if [ "$body" = web ]; then
    exit 0
  fi
  sleep 2
done
ks_log "the Ingress did not serve within 60s"
exit 1
