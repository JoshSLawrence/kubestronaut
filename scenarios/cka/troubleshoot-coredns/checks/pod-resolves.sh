#!/usr/bin/env bash
set -euo pipefail

# From a pod, the way an application would ask.
want=$(kubectl -n apps get service legacy -o 'jsonpath={.spec.clusterIP}')
out=$(kubectl -n apps exec client -- timeout 8 nslookup legacy.example.internal 2>&1 || true)
# The first "Address" line is the DNS server, the answer comes after the name.
answer=${out#*Name:}
if [ -z "$want" ] || ! grep -q "Address.*$want" <<<"$answer"; then
  echo "a pod cannot resolve legacy.example.internal to the Service address" >&2
  exit 1
fi
