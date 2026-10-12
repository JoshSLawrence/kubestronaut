#!/usr/bin/env bash
set -euo pipefail

kubectl apply -f - <<'YAML'
apiVersion: v1
kind: Service
metadata:
  name: catalog
  namespace: catalog
spec:
  selector:
    app: catalog
  ports:
    - port: 80
      targetPort: http
---
apiVersion: v1
kind: Service
metadata:
  name: catalog-np
  namespace: catalog
spec:
  type: NodePort
  selector:
    app: catalog
  ports:
    - port: 80
      targetPort: http
      nodePort: 30080
---
apiVersion: v1
kind: Service
metadata:
  name: catalog-lb
  namespace: catalog
spec:
  type: LoadBalancer
  selector:
    app: catalog
  ports:
    - port: 80
      targetPort: http
YAML
