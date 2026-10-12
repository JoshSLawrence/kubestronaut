The cluster was created without a CNI plugin, so its nodes are NotReady. The
pod network CIDR is `10.244.0.0/16`.

Two CNI manifests are on `controlplane`:

- `/opt/cni/kube-flannel.yml` (Flannel v0.28.10)
- `/opt/cni/calico.yaml` (Calico v3.33.0)

Install one of them so that all nodes become Ready, pods on different nodes
can talk to each other, and NetworkPolicies are enforced. The NetworkPolicy
in namespace `secure` must take effect. Do not modify it.
