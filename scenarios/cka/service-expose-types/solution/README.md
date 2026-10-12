# Solution

Select the pods by the Deployment's own label, `app=catalog`. The label
`tier=web` is shared with the `canary` pod, which would then receive traffic
too. `kubectl get pods --show-labels` shows both.

```bash
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
```

`kubectl expose deployment catalog --port=80 --target-port=http` builds the
first Service with the Deployment's selector. A flag cannot set a node port,
so `catalog-np` needs a manifest (or `kubectl edit`). The load balancer
implementation (MetalLB) assigns the external IP; `kubectl get svc` shows it
in the `EXTERNAL-IP` column.
