#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

# The checks compare with fingerprints taken now, so a renewal shows as a
# change whatever the certificate's dates are.
for name in apiserver kubelet-client others; do
  ks_answer_put "cert-$name" "$(bash checks/fingerprint.sh "$name")"
done
