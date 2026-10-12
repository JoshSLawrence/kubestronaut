#!/usr/bin/env bash
set -euo pipefail

kubectl apply -f - <<'YAML'
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: local-fast
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
provisioner: rancher.io/local-path
reclaimPolicy: Retain
volumeBindingMode: WaitForFirstConsumer
YAML

kubectl apply -f - <<'YAML'
apiVersion: v1
kind: Pod
metadata:
  name: cache-writer
  namespace: data
spec:
  containers:
    - name: writer
      image: busybox:1.38.0
      command: [sh, -c, "date > /cache/stamp; sleep 1d"]
      volumeMounts:
        - name: cache
          mountPath: /cache
  volumes:
    - name: cache
      persistentVolumeClaim:
        claimName: cache
YAML
kubectl -n data wait pod/cache-writer --for=condition=Ready --timeout=120s
