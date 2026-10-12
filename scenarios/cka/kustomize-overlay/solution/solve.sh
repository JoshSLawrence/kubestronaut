#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

ks_node controlplane sh -s <<'SH'
set -e
dir=/opt/kustomize/metrics-server/overlays/lab
mkdir -p "$dir"
cat > "$dir/kustomization.yaml" <<'YAML'
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../../base
labels:
  - pairs:
      app.kubernetes.io/part-of: monitoring
    includeSelectors: false
patches:
  - target:
      kind: Deployment
      name: metrics-server
    patch: |-
      - op: add
        path: /spec/template/spec/containers/0/args/-
        value: --kubelet-insecure-tls
YAML
kubectl apply -k "$dir"
SH
kubectl -n kube-system rollout status deployment/metrics-server --timeout=240s
kubectl wait --for=condition=Available apiservice/v1beta1.metrics.k8s.io --timeout=240s
