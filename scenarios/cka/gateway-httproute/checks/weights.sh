#!/usr/bin/env bash
set -euo pipefail

# One line per rule: "header names|name=weight name=weight ...". A backend
# without a weight has the API default of 1. Only a rule that does not match on
# headers counts, since requests with the canary header must not be split.
rules=$(kubectl -n shop get httproute api \
  -o 'jsonpath={range .spec.rules[*]}{.matches[*].headers[*].name}|{range .backendRefs[*]}{.name}={.weight} {end}{"\n"}{end}' 2>/dev/null || true)
while IFS='|' read -r headers line; do
  [ -z "$headers" ] || continue
  w1=
  w2=
  for pair in $line; do
    name=${pair%%=*}
    weight=${pair#*=}
    [ -n "$weight" ] || weight=1
    case "$name" in
      api-v1) w1=$weight ;;
      api-v2) w2=$weight ;;
    esac
  done
  if [ -n "$w1" ] && [ -n "$w2" ] && [ "$w2" -gt 0 ] && [ "$w1" -eq $((w2 * 9)) ]; then
    exit 0
  fi
done <<<"$rules"
echo "no rule without a header match sends to api-v1 and api-v2 in the ratio 9 to 1" >&2
exit 1
