# Solution

Flannel gives pods an overlay network but does not implement NetworkPolicy:
the objects are accepted and silently ignored, so `net-test/client` could
still reach `secure/vault`. Calico enforces them.

```bash
kubestronaut ssh controlplane
kubectl apply -f /opt/cni/calico.yaml
kubectl get nodes -w
```

Calico reads the pod CIDR from the kubeadm configuration (`10.244.0.0/16`), so
the manifest needs no edit. The nodes turn Ready once `calico-node` has
written its configuration to `/etc/cni/net.d`; then CoreDNS and the test pods
start. If the manifest is too large for client-side apply on your cluster,
use `kubectl apply --server-side -f`.

If you installed Flannel first, remove it before installing Calico, or the
two plugins fight over `/etc/cni/net.d`: `kubectl delete -f
/opt/cni/kube-flannel.yml`, delete the `10-flannel.conflist` file on every
node (`kubestronaut ssh <node>`), then re-create the pods that are already
running so that they get Calico addresses. The test pods have no controller,
so deleting them loses them; replace each one in place instead:

```bash
kubectl -n net-test get pod client -o yaml | kubectl replace --force -f -
```

Do the same for `server` in `net-test` and for `vault` and `friend` in
`secure`, and run `kubectl -n kube-system rollout restart deployment coredns`.
If that gets tangled, `kubestronaut reset` starts the scenario again.

Check the policy took effect:

```bash
kubectl -n net-test exec client -- /agnhost connect vault.secure:8080 --timeout=3s
```

Do not edit or delete the NetworkPolicy to make the connection "work": the
point of the CNI is that it blocks the connection.
