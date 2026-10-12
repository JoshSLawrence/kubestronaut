#!/usr/bin/env bash
set -euo pipefail

# A claim's size, class and access modes are immutable, so replace the claim.
# The pod is deleted first because it holds the old claim in use.
kubectl -n reports delete pod archiver --wait=true
kubectl -n reports delete pvc archive --wait=true

kubectl apply -f - <<'YAML'
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: archive
  namespace: reports
spec:
  accessModes: [ReadWriteOnce]
  storageClassName: archive
  volumeName: archive-pv
  resources:
    requests:
      storage: 2Gi
---
apiVersion: v1
kind: Pod
metadata:
  name: archiver
  namespace: reports
spec:
  containers:
    - name: archiver
      image: busybox:1.38.0
      command: [sh, -c, "sleep 1d"]
      volumeMounts:
        - name: archive
          mountPath: /archive
  volumes:
    - name: archive
      persistentVolumeClaim:
        claimName: archive
YAML
kubectl -n reports wait pod/archiver --for=condition=Ready --timeout=120s
