The cluster has three control-plane nodes behind a load balancer.
`controlplane03` lost its disk: it was wiped, and etcd still lists it as a
member. The cluster keeps working, because two of three etcd members are a
quorum.

1. Remove `controlplane03` cleanly from the cluster: its etcd member and its
   Node object.
2. Join `controlplane03` back as a control-plane node with kubeadm.

When you finish, etcd must have three healthy members.
