#!/usr/bin/env bash
set -euo pipefail

uid=$(kubectl -n data get pvc cache -o 'jsonpath={.metadata.uid}')
created=$(kubectl -n data get pvc cache \
  -o 'jsonpath={.metadata.annotations.kubestronaut\.io/created-as}')
if [ "$uid" != "$created" ]; then
  echo "claim data/cache is not the original object" >&2
  exit 1
fi
