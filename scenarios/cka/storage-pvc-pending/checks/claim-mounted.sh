#!/usr/bin/env bash
set -euo pipefail

# Tie the claim to the mount path: a claim mounted somewhere else, with another
# volume at /archive, must not count.
vol=$(kubectl -n reports get pod archiver \
  -o 'jsonpath={.spec.volumes[?(@.persistentVolumeClaim.claimName=="archive")].name}')
if [ -z "$vol" ]; then
  echo "the pod does not use the claim" >&2
  exit 1
fi
mounts=$(kubectl -n reports get pod archiver \
  -o 'jsonpath={range .spec.containers[*].volumeMounts[*]}{.name}={.mountPath}{"\n"}{end}')
if ! grep -Eqx "$vol=/archive/*" <<<"$mounts"; then
  echo "the claim is not mounted at /archive" >&2
  exit 1
fi
