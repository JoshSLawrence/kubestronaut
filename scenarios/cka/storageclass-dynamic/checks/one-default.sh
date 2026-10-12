#!/usr/bin/env bash
set -euo pipefail

defaults=$(kubectl get storageclass \
  -o 'jsonpath={range .items[?(@.metadata.annotations.storageclass\.kubernetes\.io/is-default-class=="true")]}{.metadata.name}{"\n"}{end}')
if [ "$defaults" != "local-fast" ]; then
  echo "the default StorageClass is not exactly local-fast (found: ${defaults:-none})" >&2
  exit 1
fi
