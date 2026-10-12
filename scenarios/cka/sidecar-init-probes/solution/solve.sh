#!/usr/bin/env bash
set -euo pipefail

kubectl apply -f - <<'YAML'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: legacy-app
  namespace: logging
spec:
  replicas: 1
  selector:
    matchLabels:
      app: legacy-app
  template:
    metadata:
      labels:
        app: legacy-app
    spec:
      volumes:
        - name: logs
          emptyDir: {}
      initContainers:
        - name: log-shipper
          image: busybox:1.38.0
          restartPolicy: Always
          command: [sh, -c, "tail -n+1 -F /var/log/legacy-app.log"]
          volumeMounts:
            - name: logs
              mountPath: /var/log
          securityContext:
            runAsNonRoot: true
            runAsUser: 10001
            readOnlyRootFilesystem: true
            allowPrivilegeEscalation: false
      terminationGracePeriodSeconds: 2
      containers:
        - name: app
          image: busybox:1.38.0
          command:
            - sh
            - -c
            - 'n=0; while true; do n=$((n+1)); echo "legacy-app: tick $n" >> /var/log/legacy-app.log; sleep 2; done'
          volumeMounts:
            - name: logs
              mountPath: /var/log
          readinessProbe:
            exec:
              command: [test, -f, /var/log/legacy-app.log]
            periodSeconds: 3
          securityContext:
            runAsNonRoot: true
            runAsUser: 10001
            readOnlyRootFilesystem: true
            allowPrivilegeEscalation: false
YAML
kubectl -n logging rollout status deployment/legacy-app --timeout=120s
