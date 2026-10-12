#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

# kubectl cannot create an immutable ConfigMap, so add the field to the
# generated manifest. The file is only on the node, so create it from there.
ks_node controlplane sh -c "kubectl -n config create configmap app-config \
  --from-file=/opt/config/app.properties --dry-run=client -o yaml |
  sed '/^kind: ConfigMap/a immutable: true' |
  kubectl apply -f -"

kubectl -n config create secret generic db-creds \
  --from-literal=username=app --from-literal=password='S3cure!pass'

kubectl -n config patch deployment api --type=strategic -p '
spec:
  template:
    spec:
      containers:
        - name: api
          env:
            - name: DB_USER
              valueFrom: {secretKeyRef: {name: db-creds, key: username}}
            - name: DB_PASSWORD
              valueFrom: {secretKeyRef: {name: db-creds, key: password}}
          volumeMounts:
            - name: config
              mountPath: /etc/app/app.properties
              subPath: app.properties
              readOnly: true
      volumes:
        - name: config
          configMap:
            name: app-config
'
kubectl -n config rollout status deployment/api --timeout=120s
