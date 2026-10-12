#!/usr/bin/env bash
set -euo pipefail

uid=$(kubectl -n web get deployment frontend -o 'jsonpath={.metadata.uid}')
created=$(kubectl get namespace web \
  -o 'jsonpath={.metadata.annotations.kubestronaut\.io/frontend-uid}')
if [ "$uid" != "$created" ]; then
  echo "deployment web/frontend is not the original object" >&2
  exit 1
fi
