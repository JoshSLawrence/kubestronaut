# Solution

```bash
kubectl create serviceaccount deployer -n dev
kubectl apply -f - <<'YAML'
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: deployer
  namespace: dev
rules:
  - apiGroups: [apps]
    resources: [deployments]
    verbs: [create, get, list, watch, update, patch, delete]
  - apiGroups: [""]
    resources: [pods]
    verbs: [get, list]
YAML
kubectl create rolebinding deployer -n dev --role=deployer \
  --serviceaccount=dev:deployer
kubectl create clusterrole node-reader --verb=get,list --resource=nodes
kubectl create clusterrolebinding deployer-node-reader \
  --clusterrole=node-reader --serviceaccount=dev:deployer
```

Verify with `kubectl auth can-i list nodes --as
system:serviceaccount:dev:deployer`.
