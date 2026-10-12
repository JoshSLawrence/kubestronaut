#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

# With no default class, a claim without a class stays unset and Pending.
# kind marks "standard" as the default, so take that away first.
kubectl annotate storageclass standard \
  storageclass.kubernetes.io/is-default-class- --overwrite

kubectl create namespace data
kubectl apply -f - <<'YAML'
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: cache
  namespace: data
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 100Mi
YAML

# Recreating the claim gives it a new uid, which is how the check notices.
uid=$(kubectl -n data get pvc cache -o 'jsonpath={.metadata.uid}')
kubectl -n data annotate pvc cache "kubestronaut.io/created-as=$uid"
ks_log "claim data/cache created with uid $uid"
