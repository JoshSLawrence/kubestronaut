#!/usr/bin/env bash
set -euo pipefail

# Ask a running pod that has the sidecar: during a rollout "deployment/..."
# may pick an old pod that is still shutting down.
pod=""
while read -r name inits; do
  case " $inits " in
    *" log-shipper "*) pod=$name ;;
  esac
done < <(kubectl -n logging get pods -l app=legacy-app -o \
  'jsonpath={range .items[?(@.status.phase=="Running")]}{.metadata.name}{" "}{.spec.initContainers[*].name}{"\n"}{end}')
if [ -z "$pod" ]; then
  echo "no running pod with the sidecar" >&2
  exit 1
fi
# Read the whole output first: "grep -q" exits early and would make kubectl
# fail with a broken pipe under pipefail.
out=$(kubectl -n logging logs "$pod" -c log-shipper --tail=20 2>/dev/null || true)
if ! grep -q 'legacy-app: tick' <<<"$out"; then
  echo "the sidecar does not show the application log" >&2
  exit 1
fi
