#!/usr/bin/env bash
set -euo pipefail

# Count the endpoints the control plane has published for the Service. The
# source pods matter as much as the number: the legacy pods must not be in it.
rows=$(kubectl -n store get endpointslices \
  -l kubernetes.io/service-name=checkout \
  -o 'jsonpath={range .items[*].endpoints[*]}{.conditions.ready}{" "}{.targetRef.name}{"\n"}{end}')
ready=$(grep -c '^true ' <<<"$rows" || true)
legacy=$(grep -c ' checkout-legacy-' <<<"$rows" || true)
if [ "$legacy" -ne 0 ]; then
  echo "the Service also sends traffic to pods it should not serve" >&2
  exit 1
fi
if [ "$ready" -ne 3 ]; then
  echo "the Service has $ready ready endpoints, want 3" >&2
  exit 1
fi
