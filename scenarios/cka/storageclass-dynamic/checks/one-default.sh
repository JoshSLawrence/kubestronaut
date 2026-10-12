#!/usr/bin/env bash
set -euo pipefail

# The beta annotation still marks a default class too.
defaults=$(kubectl get storageclass \
  -o 'jsonpath={range .items[*]}{.metadata.name}{" "}{.metadata.annotations.storageclass\.kubernetes\.io/is-default-class}{" "}{.metadata.annotations.storageclass\.beta\.kubernetes\.io/is-default-class}{"\n"}{end}' |
  awk '$2 == "true" || $3 == "true" {print $1}')
if [ "$defaults" != "local-fast" ]; then
  echo "the default StorageClass is not exactly local-fast (found: ${defaults:-none})" >&2
  exit 1
fi
