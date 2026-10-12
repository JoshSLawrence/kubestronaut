# Solution

`kubectl` is down, so work from the node with the container runtime:

```bash
kubestronaut ssh controlplane
crictl ps -a | grep kube-apiserver
crictl logs <container id>
```

The first log ends with an error about the key file
`/etc/kubernetes/pki/apiserver.pem`, which does not exist. `ls
/etc/kubernetes/pki` shows the key is `apiserver.key`. Fix the
`--tls-private-key-file` flag in `/etc/kubernetes/manifests/kube-apiserver.yaml`.
The kubelet restarts the static pod.

The server now starts but never becomes ready. Its log, or `/var/log/pods`,
shows connection errors to `127.0.0.1:2380`. That is etcd's peer port; clients
use 2379, as `--listen-client-urls` in `etcd.yaml` shows. Set
`--etcd-servers=https://127.0.0.1:2379`.

Do not weaken the authorization or authentication flags to get the server
running: they were not the problem.
