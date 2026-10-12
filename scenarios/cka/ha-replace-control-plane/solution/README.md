# Solution

Two things are wrong with `controlplane03`: the cluster still has an etcd
member for it, and a Node object. Remove both, then join the node again as a
control plane.

## 1. Remove the old member

On `controlplane`, with the etcd client certificates (the same ones the etcd
snapshot commands use):

```bash
export ETCDCTL_ENDPOINTS=https://127.0.0.1:2379
export ETCDCTL_CACERT=/etc/kubernetes/pki/etcd/ca.crt
export ETCDCTL_CERT=/etc/kubernetes/pki/etcd/server.crt
export ETCDCTL_KEY=/etc/kubernetes/pki/etcd/server.key
etcdctl member list
etcdctl member remove <id of the member named after controlplane03>
kubectl delete node <controlplane03's node name>
```

Do this first. If you join before removing the member, the old member is
still registered for the same node. The fresh etcd on the wiped disk
conflicts with it and crashes, and the join hangs until its timeouts expire.

## 2. Join it as a control plane

A control-plane join needs the cluster certificates as well as a token.
`upload-certs` stores them encrypted in the cluster and prints the key. On
`controlplane`:

```bash
kubeadm init phase upload-certs --upload-certs
kubeadm token create --print-join-command --certificate-key <key>
```

Run the printed command (it ends in `--control-plane --certificate-key ...`)
on `controlplane03`:

```bash
kubestronaut ssh controlplane03
kubeadm join <load balancer>:6443 --token ... \
  --discovery-token-ca-cert-hash sha256:... \
  --control-plane --certificate-key ...
```

Without `--control-plane` the node joins as a worker and etcd stays at two
members. The uploaded certificates are deleted after two hours.

## Notes on the lab

The three control planes sit behind kind's load balancer, a separate
container named `<cluster>-external-load-balancer`; `controlPlaneEndpoint`
points at it, so the join command reaches whichever API server is up.

Setup runs `kubeadm reset -f --skip-phases=remove-etcd-member` on
`controlplane03` and deletes `/var/lib/etcd`, which is what a lost disk
leaves behind: no kubelet configuration, no static pods and no data, and the
old member still in etcd.

## Spike results

Measured on kind v0.33.0 (arm64, Docker Desktop with 11.7 GiB):

- Creating three control planes, a worker and the load balancer takes about
  70 seconds.
- Memory at steady state is about 1.9 GiB for the five containers (control
  planes 0.5 to 0.7 GiB each, the worker 0.15 GiB, the load balancer 30 MiB).
  The join does not raise it.
- With one member down, the API server answers through the load balancer
  and `etcdctl endpoint health --cluster` reports the dead endpoint
  unhealthy.
- After `member remove` and `kubectl delete node`, the join takes about 15
  seconds without any preflight flag. The node is Ready right away and its
  etcd and API server pods are Running.
