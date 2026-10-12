#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

# The key lives on the node, so the request is built there.
csr=$(ks_node controlplane sh -c \
  'openssl req -new -key /root/jane.key -subj /CN=jane | base64 | tr -d "\n"')

kubectl apply -f - <<YAML
apiVersion: certificates.k8s.io/v1
kind: CertificateSigningRequest
metadata:
  name: jane
spec:
  request: $csr
  signerName: kubernetes.io/kube-apiserver-client
  usages: [client auth]
YAML
kubectl certificate approve jane

for _ in $(seq 20); do
  cert=$(kubectl get csr jane -o 'jsonpath={.status.certificate}')
  [ -n "$cert" ] && break
  sleep 2
done
printf '%s' "$cert" | base64 -d | ks_node controlplane sh -c 'cat > /root/jane.crt'

kubectl -n finance create role pod-reader --verb=get,list,watch --resource=pods
kubectl -n finance patch role pod-reader --type=json -p \
  '[{"op":"add","path":"/rules/-","value":{"apiGroups":[""],"resources":["pods/log"],"verbs":["get"]}}]'
kubectl -n finance create rolebinding jane-pod-reader --role=pod-reader --user=jane
