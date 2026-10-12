#!/usr/bin/env bash
set -euo pipefail

# The file may be mounted as the directory /etc/app or as a single file via
# subPath, even both; every such mount has to be read-only.
mounts=$(kubectl -n config get deployment api \
  -o 'jsonpath={range .spec.template.spec.containers[*].volumeMounts[*]}{.mountPath}{" "}{.readOnly}{"\n"}{end}')
rows=$(sed -E 's#/+ # #' <<<"$mounts" | awk '
  $1 == "/etc/app" || $1 == "/etc/app/app.properties" {print}')
if [ -z "$rows" ]; then
  echo "no mount provides /etc/app/app.properties" >&2
  exit 1
fi
if awk '$2 != "true" {bad = 1} END {exit !bad}' <<<"$rows"; then
  echo "the file is not mounted read-only" >&2
  exit 1
fi
