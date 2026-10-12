#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

# The files are written from here, not shipped as manifests, so they are not
# applied by setup and do not stay in the learner's scenario directory.
dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT

cat >"$dir/policy-a.yaml" <<'YAML'
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-clients
  namespace: backend
spec:
  podSelector:
    matchLabels:
      app: backend
  policyTypes: [Ingress]
  ingress:
    - from:
        - namespaceSelector: {}
YAML

cat >"$dir/policy-b.yaml" <<'YAML'
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-frontend
  namespace: backend
spec:
  podSelector:
    matchLabels:
      app: backend
  policyTypes: [Ingress]
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: frontend
          podSelector:
            matchLabels:
              app: frontend
      ports:
        - protocol: TCP
          port: 8080
YAML

cat >"$dir/policy-c.yaml" <<'YAML'
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-frontend-ns
  namespace: backend
spec:
  podSelector:
    matchLabels:
      app: backend
  policyTypes: [Ingress]
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: frontend
      ports:
        - protocol: TCP
          port: 80
YAML

ks_log "copying the candidate policies to /root/netpol on controlplane"
ks_copy_to controlplane "$dir" /root/netpol
ks_node controlplane test -s /root/netpol/policy-b.yaml
