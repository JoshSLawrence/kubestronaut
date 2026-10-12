# Solution

`kubectl -n reports describe pvc archive` shows why nothing binds: the claim
asks for ReadWriteMany, 5Gi and the `standard` class, while `archive-pv` is
ReadWriteOnce, 2Gi and class `archive`. A claim's spec cannot be edited, so
delete it and create a matching one. Setting `volumeName` pins it to the
volume, and the volume's data is kept because it uses `Retain` and a
`hostPath`.

```bash
kubectl -n reports delete pod archiver
kubectl -n reports delete pvc archive
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
YAML
```

Then recreate the pod with the same claim mounted at `/archive`. Leaving out
`volumeName` also works: the claim matches `archive-pv` by class, mode and
size. Editing `archive-pv` to fit the claim, or creating another volume,
loses the original data or changes what the task protects.
