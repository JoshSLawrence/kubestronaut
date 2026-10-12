#!/usr/bin/env bash
set -euo pipefail

kubectl apply -f - <<'YAML'
apiVersion: v1
kind: PersistentVolume
metadata:
  name: pv-logs
spec:
  capacity:
    storage: 1Gi
  accessModes: [ReadWriteOnce]
  storageClassName: manual
  persistentVolumeReclaimPolicy: Retain
  hostPath:
    path: /mnt/logs
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: logs
  namespace: app
spec:
  accessModes: [ReadWriteOnce]
  storageClassName: manual
  resources:
    requests:
      storage: 1Gi
---
apiVersion: v1
kind: Pod
metadata:
  name: logger
  namespace: app
spec:
  containers:
    - name: logger
      image: busybox:1.38.0
      command: [sh, -c, "date >> /var/log/app/out.log; sleep 1d"]
      volumeMounts:
        - name: logs
          mountPath: /var/log/app
  volumes:
    - name: logs
      persistentVolumeClaim:
        claimName: logs
YAML
kubectl -n app wait pod/logger --for=condition=Ready --timeout=120s
