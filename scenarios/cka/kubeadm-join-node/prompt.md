`node01` was removed from the cluster and reset. containerd, kubeadm and the
kubelet are installed on it, and `controlplane` is healthy.

1. On `controlplane`, create a bootstrap token that expires in **2 hours**.
   Write the complete `kubeadm join` command that uses this token to
   `/root/node01-join.sh`.
2. Join `node01` to the cluster as a worker node with that command.
   Deployment `edge/pinned` must run on `node01`.
