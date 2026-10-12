# Solution

`kubectl get priorityclass` shows the classes; the highest user-defined one is
`tier-gold` at 750000 (the `system-*` classes are built in). Create the new
class one below it:

```bash
kubectl apply -f - <<'YAML'
apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
metadata:
  name: high-priority
value: 749999
globalDefault: false
YAML

kubectl -n prod patch deployment api --type=strategic -p \
  '{"spec":{"template":{"spec":{"priorityClassName":"high-priority"}}}}'
```

With a higher priority than the batch pods, the scheduler preempts one of
them to make room for `api`. The deployment then recreates that batch pod, but
it stays Pending because the node is full and it cannot preempt anything.
