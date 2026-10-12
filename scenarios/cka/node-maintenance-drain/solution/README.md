# Solution

`kubectl drain` cordons the node and evicts its pods. Three things need care:

- DaemonSet pods cannot be evicted, so pass `--ignore-daemonsets`.
- `cache` stores data in an `emptyDir`, which is lost on eviction, so drain
  asks for `--delete-emptydir-data`.
- `scratch` has no controller, so nothing recreates it after eviction and
  drain refuses it unless you pass `--force`, which deletes it for good. Save
  its definition first, and create it again afterwards without `nodeName`
  (a pod with `nodeName` set bypasses the scheduler, and so the cordon).

```bash
kubectl -n maint get pod scratch -o yaml > scratch.yaml
# edit scratch.yaml: remove nodeName, status and other runtime fields
kubectl drain node01 --ignore-daemonsets --delete-emptydir-data --force
kubectl apply -f scratch.yaml
```

The drain respects the PodDisruptionBudget: it evicts `web` pods one at a
time and waits until each replacement is ready on the other node, so the
deployment keeps at least 2 available pods. Deleting the budget or the pods
would be faster, and is exactly what the task forbids.
