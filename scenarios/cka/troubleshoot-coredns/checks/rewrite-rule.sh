#!/usr/bin/env bash
set -euo pipefail

# Several Corefile spellings map a name to another name (rewrite with or
# without "stop"/"exact", a trailing dot, "answer auto", the block form, a
# template with a CNAME), so check the meaning instead of the syntax: the
# Corefile must point at the Service's DNS name, and must not hold the
# Service's address, which goes stale when the Service is recreated.
ip=$(kubectl -n apps get service legacy -o 'jsonpath={.spec.clusterIP}')
corefile=$(kubectl -n kube-system get configmap coredns -o 'jsonpath={.data.Corefile}')
active=$(grep -v '^[[:space:]]*#' <<<"$corefile" || true)
if [ -n "$ip" ] && grep -Fq "$ip" <<<"$active"; then
  echo "the Corefile holds an address that goes stale when the Service changes" >&2
  exit 1
fi
if ! grep -Fq 'legacy.apps.svc.cluster.local' <<<"$active"; then
  echo "the Corefile does not map the name to the Service's DNS name" >&2
  exit 1
fi
