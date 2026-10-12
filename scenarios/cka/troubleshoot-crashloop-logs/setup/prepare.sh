#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

# The id appears in the last log line, so only a learner who reads the logs can
# produce it. It lives in a Secret so that it is the same after every restart.
id=$(od -An -N4 -tx1 /dev/urandom | tr -d ' \n')
kubectl create namespace orders
kubectl -n orders create secret generic orders-runtime \
  --from-literal=instance-id="$id"
