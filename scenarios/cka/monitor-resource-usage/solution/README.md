# Solution

`kubectl top` reads metrics-server. Both listings can be sorted, which puts
the heaviest first:

```bash
kubectl top pods -n metrics -l app=batch --sort-by=cpu
kubectl top nodes --sort-by=memory
```

Write the first name of each to the files on `controlplane`:

```bash
kubectl top pods -n metrics -l app=batch --no-headers --sort-by=cpu \
  | head -n 1 | awk '{print $1}' > /root/top-pod.txt
kubectl top nodes --no-headers --sort-by=memory \
  | head -n 1 | awk '{print $1}' > /root/top-node.txt
```

Without the label selector, the busier `report-builder` pod comes first. The
busiest batch pod is different in every session.
