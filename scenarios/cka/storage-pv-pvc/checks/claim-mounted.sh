#!/usr/bin/env bash
set -euo pipefail

# Tie the claim to the mount path: a claim mounted somewhere else, with another
# volume at /var/log/app, must not count.
vol=$(kubectl -n app get pod logger \
  -o 'jsonpath={.spec.volumes[?(@.persistentVolumeClaim.claimName=="logs")].name}')
if [ -z "$vol" ]; then
  echo "the pod does not use the claim" >&2
  exit 1
fi
mounts=$(kubectl -n app get pod logger \
  -o 'jsonpath={range .spec.containers[*].volumeMounts[*]}{.name}={.mountPath}{"\n"}{end}')
if ! grep -Eqx "$vol=/var/log/app/*" <<<"$mounts"; then
  echo "the claim is not mounted at /var/log/app" >&2
  exit 1
fi
