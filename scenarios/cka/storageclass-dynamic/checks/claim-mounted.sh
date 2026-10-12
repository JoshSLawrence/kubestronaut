#!/usr/bin/env bash
set -euo pipefail

# Tie the claim to the mount path: a claim mounted somewhere else, with another
# volume at /cache, must not count.
vol=$(kubectl -n data get pod cache-writer \
  -o 'jsonpath={.spec.volumes[?(@.persistentVolumeClaim.claimName=="cache")].name}')
if [ -z "$vol" ]; then
  echo "the pod does not use the claim" >&2
  exit 1
fi
mounts=$(kubectl -n data get pod cache-writer \
  -o 'jsonpath={range .spec.containers[*].volumeMounts[*]}{.name}={.mountPath}{"\n"}{end}')
if ! grep -Eqx "$vol=/cache/*" <<<"$mounts"; then
  echo "the claim is not mounted at /cache" >&2
  exit 1
fi
