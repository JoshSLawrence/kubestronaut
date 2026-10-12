# Solution

On `controlplane`, write the report, then renew the two certificates. Each
certificate has its own `renew` subcommand; `renew all` would also change the
etcd, front proxy and kubeconfig certificates, which the task forbids.

```bash
kubestronaut ssh controlplane
kubeadm certs check-expiration > /root/cert-expiry.txt
kubeadm certs renew apiserver
kubeadm certs renew apiserver-kubelet-client
```

Renewing only rewrites the files; the task asks for a restart so that the
running server loads them. Restart the static pod: move its manifest out of
`/etc/kubernetes/manifests` and back, or stop its container with `crictl`:

```bash
mv /etc/kubernetes/manifests/kube-apiserver.yaml /tmp/
sleep 20
mv /tmp/kube-apiserver.yaml /etc/kubernetes/manifests/
```

`openssl s_client -connect 127.0.0.1:6443` shows the certificate the running
server presents.
