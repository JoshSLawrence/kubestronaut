#!/usr/bin/env bash
set -euo pipefail

deploy=$(kubectl -n kube-system get deployment metrics-server \
  -o 'jsonpath={.spec.selector.matchLabels}' 2>/dev/null || true)
service=$(kubectl -n kube-system get service metrics-server \
  -o 'jsonpath={.spec.selector}' 2>/dev/null || true)
if [ -z "$deploy" ] || [ -z "$service" ]; then
  echo "metrics-server is not installed" >&2
  exit 1
fi
case "$deploy$service" in
  *part-of*)
    echo "a selector gained the part-of label" >&2
    exit 1
    ;;
esac
