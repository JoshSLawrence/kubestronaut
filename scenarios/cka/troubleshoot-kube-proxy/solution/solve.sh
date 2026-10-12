#!/usr/bin/env bash
set -euo pipefail

kubectl -n kube-system patch daemonset kube-proxy --type=json -p \
  '[{"op":"replace","path":"/spec/template/spec/containers/0/command/1","value":"--config=/var/lib/kube-proxy/config.conf"}]'
kubectl -n kube-system rollout status daemonset/kube-proxy --timeout=240s
kubectl -n shop exec client -- nc -z -w 3 api.shop.svc.cluster.local 8080
