The cluster runs Kubernetes v1.36.4 on `controlplane` and `node01`. Upgrade
both nodes to exactly **v1.37.1**.

- Use the packages from the apt repository configured on the nodes. It is a
  local mirror of the v1.37 package line.
- Upgrade kubeadm first, then the control plane, then the kubelet and kubectl.
- Drain each node before you upgrade its kubelet, and make it schedulable
  again afterwards.
- Deployment `apps/web` must be available when you finish.

Use `kubestronaut ssh controlplane` and `kubestronaut ssh node01` to reach the
nodes.
