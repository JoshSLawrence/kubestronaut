#!/usr/bin/env bash
set -euo pipefail

# Run the learner's probe command against the real file, which must succeed,
# and against a file that does not exist, which must fail. That tells a probe
# that tests the file from one that only mentions it.
# Prefer a pod from the latest revision (the one with the sidecar), but do not
# make this check depend on the sidecar being right.
pod=""
while read -r name inits; do
  case " $inits " in
    *" log-shipper "*) pod=$name ;;
    *) [ -n "$pod" ] || pod=$name ;;
  esac
done < <(kubectl -n logging get pods -l app=legacy-app -o \
  'jsonpath={range .items[?(@.status.phase=="Running")]}{.metadata.name}{" "}{.spec.initContainers[*].name}{"\n"}{end}')
if [ -z "$pod" ]; then
  echo "no running pod" >&2
  exit 1
fi
mapfile -t cmd < <(kubectl -n logging get deployment legacy-app -o \
  'jsonpath={range .spec.template.spec.containers[?(@.name=="app")].readinessProbe.exec.command[*]}{@}{"\n"}{end}')
if [ "${#cmd[@]}" -eq 0 ]; then
  echo "the app container has no exec readiness probe" >&2
  exit 1
fi
if ! grep -q 'legacy-app\.log' <<<"${cmd[*]}"; then
  echo "the readiness probe does not look at the log file" >&2
  exit 1
fi
missing=("${cmd[@]//legacy-app.log/ks-missing.log}")
if ! kubectl -n logging exec "$pod" -c app -- "${cmd[@]}" >/dev/null 2>&1; then
  echo "the readiness probe fails while the log file exists" >&2
  exit 1
fi
if kubectl -n logging exec "$pod" -c app -- "${missing[@]}" >/dev/null 2>&1; then
  echo "the readiness probe succeeds while the log file is missing" >&2
  exit 1
fi
