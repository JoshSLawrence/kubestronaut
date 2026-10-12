# Solution

Pods cannot reach DNS at all, so start with the Service:

```bash
kubectl -n kube-system get svc,endpointslices -l k8s-app=kube-dns
kubectl -n kube-system get pods --show-labels
```

1. The `kube-dns` Service has no endpoints: its selector is `k8s-app: kubedns`
   but the CoreDNS pods are labelled `k8s-app: kube-dns`.

   ```bash
   kubectl -n kube-system patch svc kube-dns --type=merge \
     -p '{"spec":{"selector":{"k8s-app":"kube-dns"}}}'
   ```

2. With endpoints back, `dig @10.96.0.10 kubernetes.default.svc.cluster.local`
   from a node still returns nothing. The Corefile in the `coredns` ConfigMap
   says `kubernetes cluster.locl`: the zone is misspelled. Fix it to
   `cluster.local`.

3. Add the custom name inside the `.:53` block of the same Corefile. A
   `rewrite` makes CoreDNS answer for the Service's real name, so the answer
   follows the Service even if its IP changes:

   ```text
   rewrite name legacy.example.internal legacy.apps.svc.cluster.local
   ```

   Do not use a `hosts` block with the current IP: it goes stale.

4. Apply the ConfigMap and restart CoreDNS so it reads it at once:

   ```bash
   kubectl -n kube-system rollout restart deployment/coredns
   ```
