# Solution

```bash
kubectl -n web rollout undo deployment/frontend
kubectl -n web scale deployment/frontend --replicas=5
kubectl -n web patch deployment/frontend --type=merge -p \
  '{"spec":{"strategy":{"type":"RollingUpdate","rollingUpdate":{"maxSurge":1,"maxUnavailable":0}}}}'
kubectl -n web set resources deployment/frontend \
  --requests=cpu=50m,memory=64Mi
kubectl -n web rollout status deployment/frontend --timeout=120s
```

`rollout undo` goes back to the previous revision, which is the last one that
worked. With 5 replicas, `maxUnavailable: 0` means all 5 old pods stay ready
until a new one replaces them, and `maxSurge: 1` allows one extra pod at a
time. Changing the pod template with `set resources` starts a new rollout, so
the new requests only reach the pods once that rollout finishes.
