#!/usr/bin/env bash
set -euo pipefail

tmpl='.spec.template.spec'
# mounts <containers|initContainers> <name>: print "volume=path" lines.
mounts() {
  kubectl -n logging get deployment legacy-app -o \
    "jsonpath={range ${tmpl}.$1[?(@.name==\"$2\")].volumeMounts[*]}{.name}={.mountPath}{\"\\n\"}{end}"
}

# Tie the volume to the mount path in both containers: a volume named logs that
# is mounted somewhere else, or a different volume at /var/log, must not count.
if [ -z "$(kubectl -n logging get deployment legacy-app \
  -o "jsonpath={${tmpl}.volumes[?(@.name==\"logs\")].emptyDir}")" ]; then
  echo "there is no emptyDir volume named logs" >&2
  exit 1
fi
if ! grep -Eqx 'logs=/var/log/*' <<<"$(mounts containers app)" ||
  ! grep -Eqx 'logs=/var/log/*' <<<"$(mounts initContainers log-shipper)"; then
  echo "the volume is not mounted at /var/log in both containers" >&2
  exit 1
fi
