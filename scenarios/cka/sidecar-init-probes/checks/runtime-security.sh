#!/usr/bin/env bash
set -euo pipefail

# Look at what the running containers really got, in a pod that has the
# sidecar, so an edit that was never rolled out does not count.
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
# Runs inside each container. /tmp is world-writable in the image, so a failed
# write there means the root filesystem is read-only, not a permission problem.
probe=$(
  cat <<'SH'
echo "uid=$(id -u)"
grep NoNewPrivs /proc/self/status
if touch /tmp/ks-write-test 2>/dev/null; then
  echo rootfs=writable
else
  echo rootfs=readonly
fi
SH
)
for c in app log-shipper; do
  out=$(kubectl -n logging exec "$pod" -c "$c" -- sh -c "$probe" 2>/dev/null) ||
    {
      echo "cannot run a command in container $c" >&2
      exit 1
    }
  grep -qx 'uid=10001' <<<"$out" || {
    echo "container $c does not run as user 10001" >&2
    exit 1
  }
  grep -Eq 'NoNewPrivs:[[:space:]]+1' <<<"$out" || {
    echo "container $c can gain privileges" >&2
    exit 1
  }
  grep -qx 'rootfs=readonly' <<<"$out" || {
    echo "container $c has a writable root filesystem" >&2
    exit 1
  }
done
