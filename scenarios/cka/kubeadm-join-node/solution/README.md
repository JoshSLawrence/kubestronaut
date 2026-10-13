# Solution

`kubeadm token create` makes a bootstrap token, and `--print-join-command`
prints the complete command for a node: the API endpoint, the token and the
hash of the cluster CA that the node pins. `--ttl 2h` sets the lifetime (the
default is 24 hours).

On `controlplane`:

```bash
kubeadm token create --ttl 2h --print-join-command | tee /root/node01-join.sh
```

On `node01`, run the printed command as root:

```bash
kubestronaut ssh node01
kubeadm join <endpoint>:6443 --token <id>.<secret> \
  --discovery-token-ca-cert-hash sha256:<hash>
```

Without `--print-join-command` you can build the command yourself. The CA
hash comes from the CA certificate, and `kubeadm token list` shows the token:

```bash
openssl x509 -pubkey -in /etc/kubernetes/pki/ca.crt |
  openssl rsa -pubin -outform der | sha256sum
```

The kubelet then performs the TLS bootstrap, the node registers, and the
DaemonSets (kube-proxy and the CNI) start on it. `edge/pinned` selects
`node01` by hostname, so its pod stays Pending until the node is back.

## Notes on the lab

Setup drains `node01`, runs `kubeadm reset -f` on it and deletes the Node
object. kind keeps its own join token, `abcdef.0123456789abcdef`, which
never expires; setup deletes it so that only a token you create works.

Spike result (kind v0.33.0, Kubernetes v1.36.4, arm64): `kubeadm join` needs
no extra flags. It prints one preflight warning (`Swap`), which is harmless
here. The node is Ready about 10 seconds after the join, its InternalIP is
the container's address, and cross-node pod traffic works. kind's kubelet
settings (`cgroupRoot: /kubelet`, `failSwapOn: false`) come back from the
`kubelet-config` ConfigMap during the join.
