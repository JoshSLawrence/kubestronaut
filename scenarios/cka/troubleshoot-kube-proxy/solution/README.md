# Solution

Services are implemented on every node by kube-proxy, so look at its pods:

```bash
kubectl -n kube-system get pods -l k8s-app=kube-proxy
kubectl -n kube-system logs ds/kube-proxy
```

The pods crash because the DaemonSet starts kube-proxy with
`--config=/var/lib/kube-proxy/kubeproxy.conf`. The ConfigMap mounted at
`/var/lib/kube-proxy` holds `config.conf`, not that file. Put the right file
name back:

```bash
kubectl -n kube-system edit daemonset kube-proxy
```

The pods restart, kube-proxy programs the Service rules again
(`iptables-save -t nat | grep KUBE-SVC` on a node shows them) and Services
and cluster DNS work.

Writing the iptables rules by hand, or switching the proxy mode in the
ConfigMap, would not fix the DaemonSet and does not survive a restart.
