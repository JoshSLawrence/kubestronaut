# Solution

Follow the order of the Kubernetes guide "Upgrading kubeadm clusters": the
control plane first, then each other node. The apt repository on the nodes
is a local mirror of the v1.37 line (`apt-cache policy kubeadm` shows
`1.37.1-1.1`).

On `controlplane` (`kubestronaut ssh controlplane`):

```bash
apt-get update
apt-mark unhold kubeadm
apt-get install -y kubeadm=1.37.1-1.1
apt-mark hold kubeadm
kubeadm upgrade plan
kubeadm upgrade apply v1.37.1
```

`apply` upgrades the static pods (etcd, API server, controller manager and
scheduler), the cluster configuration and the add-ons (kube-proxy and
CoreDNS). It does not touch the kubelet. Drain the node, replace the kubelet
and kubectl, restart the kubelet and make the node schedulable again:

```bash
kubectl drain controlplane --ignore-daemonsets --delete-emptydir-data
apt-mark unhold kubelet kubectl
apt-get install -y kubelet=1.37.1-1.1 kubectl=1.37.1-1.1
apt-mark hold kubelet kubectl
systemctl daemon-reload
systemctl restart kubelet
kubectl uncordon controlplane
```

Use the real node name for `controlplane` in `kubectl drain`. On `node01`
install kubeadm the same way, then run `kubeadm upgrade node` (not
`apply`, which is for control-plane nodes), drain the node from the control
plane, install the kubelet, restart it and uncordon.

Restarting the upgraded kubelet makes it recreate the control-plane static
pods, so the API server restarts once more. Wait until `kubectl` answers
steadily before you drain the next node; a drain started while the API server
is restarting fails with `forbidden` errors.

## What the grade looks at

- the packages, with `dpkg-query`, so copying binaries does not pass;
- `kubernetesVersion` in the `kubeadm-config` ConfigMap, the static pod
  manifests, and the add-on images, which only `kubeadm upgrade apply`
  changes;
- the kubelet version of both nodes, and that both are schedulable and Ready;
- that the pods of `apps/web` were recreated, which only a drain (or another
  eviction) does.

## Notes on the lab

- kind installs the Kubernetes binaries outside dpkg. The node image adds the
  v1.37.1 `.deb` files as a local repository and a small package that
  provides `kubernetes-cni` (kind ships its CNI plugins, v1.9.1, outside
  dpkg). `apt-mark unhold kubeadm` therefore prints "kubeadm was already not
  on hold" before the first install, and that is fine.
- kind's kubelet unit files live in `/etc/systemd/system`, which shadows the
  ones in the deb, and dpkg keeps kind's `/etc/default/kubelet`
  (`--runtime-cgroups`), so the new kubelet starts with kind's settings.
- The control-plane images of v1.37.1 are loaded into the nodes when the
  cluster is created, which needs internet the first time (Docker pulls them).
  If one could not be loaded, setup tells you; `apply` then needs internet
  to fetch it.

## Spike results

Measured on kind v0.33.0, arm64, Docker Desktop with 11.7 GiB:

- The offline repository works with the network disabled: `apt-get update` and
  `apt-get install kubeadm=1.37.1-1.1 kubelet=1.37.1-1.1 kubectl=1.37.1-1.1`
  succeed, `/opt/cni/bin` stays unchanged, and the layer adds 32.7 MB.
- `kubeadm upgrade apply v1.37.1` needs no `--ignore-preflight-errors` and
  takes about 160 seconds. etcd goes from 3.6.8 to 3.7.0 without trouble, so
  `--etcd-upgrade=false` is not needed.
- `kubeadm upgrade plan` suggests v1.36.5 and v1.37.1 when the machine has
  internet (it looks up the newest patch release), and falls back to the
  client version without it.
- After the upgrade `cgroupRoot: /kubelet`, `failSwapOn: false` and the
  kube-proxy `conntrack.maxPerCore: 0` of kind are still in place, and the
  kubelet keeps `--node-ip` and `--provider-id`.
- Memory peaks at about 0.97 GiB for the two nodes.
- `kubeadm upgrade node` on the worker takes a few seconds.
