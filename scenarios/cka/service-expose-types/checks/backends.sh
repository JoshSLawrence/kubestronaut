#!/usr/bin/env bash
set -euo pipefail

# $1 is the Service. Its EndpointSlices must list exactly the two ready pods of
# Deployment catalog, nothing else (the decoy pod carries the same tier label).
svc=$1
rows=$(kubectl -n catalog get endpointslices \
  -l "kubernetes.io/service-name=$svc" \
  -o 'jsonpath={range .items[*].endpoints[*]}{.conditions.ready} {.targetRef.name}{"\n"}{end}')
total=0
while read -r ready name; do
  [ "$ready" = true ] || continue
  if ! [[ $name =~ ^catalog-[a-z0-9]+-[a-z0-9]+$ ]]; then
    echo "Service $svc sends to $name, which is not a pod of Deployment catalog" >&2
    exit 1
  fi
  total=$((total + 1))
done <<<"$rows"
if [ "$total" -ne 2 ]; then
  echo "Service $svc has $total ready catalog endpoints, want 2" >&2
  exit 1
fi
