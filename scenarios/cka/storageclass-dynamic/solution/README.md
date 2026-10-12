# Solution

```bash
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
```

The default class is an annotation; `kubectl get storageclass` shows which
class carries `(default)`. A claim created while no class was the default has
no class at all, and the cluster fills it in with the new default afterwards,
so the claim does not need to be recreated. `WaitForFirstConsumer` delays
provisioning until a pod uses the claim, which is why the claim stays Pending
until `cache-writer` exists.
