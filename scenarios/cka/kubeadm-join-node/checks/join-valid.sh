#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

# Everything runs on controlplane, which has the admin kubeconfig, GNU date
# and openssl. Prints one line on the first thing that is wrong.
read -r -d '' remote <<'REMOTE' || true
set -u
f=/root/node01-join.sh
# Accept a command wrapped over several lines with trailing backslashes.
line=$(tr '\n' ' ' <"$f" | sed 's/\\ */ /g')
case "$line" in
  *'kubeadm join'*) ;;
  *)
    echo "the file holds no kubeadm join command"
    exit 0
    ;;
esac
token=$(printf '%s\n' "$line" | sed -n 's/.*--token[ =]\([a-z0-9]\{6\}\.[a-z0-9]\{16\}\).*/\1/p')
hash=$(printf '%s\n' "$line" | sed -n 's/.*--discovery-token-ca-cert-hash[ =]sha256:\([a-f0-9]\{64\}\).*/\1/p')
endpoint=$(printf '%s\n' "$line" | sed -n 's/.*kubeadm join \([^ ]*:6443\).*/\1/p')
if [ -z "$token" ] || [ -z "$hash" ] || [ -z "$endpoint" ]; then
  echo "the join command lacks an endpoint, a token or a CA hash"
  exit 0
fi
id=${token%%.*}
secret=${token#*.}
if [ "$id" = abcdef ]; then
  echo "the token is not one you created"
  exit 0
fi
stored=$(kubectl -n kube-system get secret "bootstrap-token-$id" -o 'jsonpath={.data.token-secret}' 2>/dev/null | base64 -d 2>/dev/null || true)
if [ "$stored" != "$secret" ]; then
  echo "the token in the command does not exist in the cluster"
  exit 0
fi
expiry=$(kubectl -n kube-system get secret "bootstrap-token-$id" -o 'jsonpath={.data.expiration}' | base64 -d)
created=$(kubectl -n kube-system get secret "bootstrap-token-$id" -o 'jsonpath={.metadata.creationTimestamp}')
ttl=$(($(date -d "$expiry" +%s) - $(date -d "$created" +%s)))
if [ "$ttl" -lt 7080 ] || [ "$ttl" -gt 7320 ]; then
  echo "the token's lifetime is not 2 hours"
  exit 0
fi
want=$(openssl x509 -pubkey -in /etc/kubernetes/pki/ca.crt | openssl rsa -pubin -outform der 2>/dev/null | sha256sum | cut -d' ' -f1)
if [ "$hash" != "$want" ]; then
  echo "the CA hash does not match the cluster CA"
  exit 0
fi
cfg=$(kubectl -n kube-system get cm kubeadm-config -o 'jsonpath={.data.ClusterConfiguration}' | sed -n 's/^controlPlaneEndpoint: //p')
ips=$(kubectl get nodes -l node-role.kubernetes.io/control-plane -o 'jsonpath={.items[*].status.addresses[?(@.type=="InternalIP")].address}')
ok=false
[ "$endpoint" = "$cfg" ] && ok=true
for ip in $ips; do
  [ "$endpoint" = "$ip:6443" ] && ok=true
done
if [ "$ok" != true ]; then
  echo "the endpoint is not the cluster's control plane endpoint"
  exit 0
fi
echo OK
REMOTE
out=$(printf '%s\n' "$remote" | ks_node controlplane bash -s || true)
# The remote script ends with OK only when every test passed, so a command
# that failed to run cannot pass as an empty report.
if [ "$(printf '%s\n' "$out" | tail -n 1)" != OK ]; then
  echo "${out:-the join file could not be checked on controlplane}" >&2
  exit 1
fi
