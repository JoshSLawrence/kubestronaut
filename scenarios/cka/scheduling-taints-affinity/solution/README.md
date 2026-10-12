# Solution

Edit the deployment with `kubectl -n ml edit deployment trainer` and add to
the pod template's `spec`:

```yaml
tolerations:
  - key: dedicated
    operator: Equal
    value: gpu
    effect: NoSchedule
affinity:
  nodeAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      nodeSelectorTerms:
        - matchExpressions:
            - key: accelerator
              operator: In
              values: [gpu]
```

The toleration only lets the pods onto the tainted node; it does not attract
them. Without the required node affinity (or a `nodeSelector`) the scheduler
may still place them on the untainted node. Use the node's real name in a
`nodeName` field and you bypass the scheduler, which the task rules out.
