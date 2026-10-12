#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

# The probe reads the Service address from its environment, which kubelet only
# fills in for Services that existed when the pod started.
kubectl -n infra exec probe -- printenv ECHO_SERVICE_HOST >/dev/null

ks_log "turning IPv4 forwarding off and pointing crictl at the wrong runtime"
ks_node node01 sysctl -w net.ipv4.ip_forward=0
ks_node node01 tee /etc/crictl.yaml >/dev/null <<'YAML'
runtime-endpoint: unix:///run/crio/crio.sock
image-endpoint: unix:///run/crio/crio.sock
timeout: 10
YAML

# Nothing on the node may turn forwarding back on by itself, or the first
# grade would not be stable.
sleep 8
value=$(ks_node node01 sysctl -n net.ipv4.ip_forward)
if [ "$value" != 0 ]; then
  ks_log "net.ipv4.ip_forward was reset to $value within 8s"
  exit 1
fi
