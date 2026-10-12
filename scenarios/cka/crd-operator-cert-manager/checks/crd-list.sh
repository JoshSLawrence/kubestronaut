#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

# What is installed now. Judged against the live cluster, so the answer
# follows the manifest rather than a list that could go stale.
want=$(kubectl get crd -o name 2>/dev/null | sed 's#.*/##' | grep -E '(^|\.)cert-manager\.io$' | sort || true)
if [ -z "$want" ]; then
  echo "cert-manager's CRDs are not installed" >&2
  exit 1
fi
# Accept bare names and kubectl's "kind.group/name" form.
got=$(ks_node controlplane cat /root/cert-manager-crds.txt 2>/dev/null |
  sed -e 's#.*/##' -e 's/[[:space:]]*$//' | grep -v '^$' | sort || true)
if [ "$got" != "$want" ]; then
  echo "/root/cert-manager-crds.txt does not list exactly the CRDs of cert-manager" >&2
  exit 1
fi
