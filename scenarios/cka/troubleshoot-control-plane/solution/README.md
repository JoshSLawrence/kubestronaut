# Solution

Nothing creates the deployment's pods and nothing schedules them, so look at
the two components behind that:

```bash
kubectl -n kube-system get pods
kubestronaut ssh controlplane
ls /etc/kubernetes /etc/kubernetes/manifests
crictl ps -a
crictl logs <controller-manager container id>
```

- The scheduler has no pod: its static pod manifest is not in
  `/etc/kubernetes/manifests`. A backup, `/etc/kubernetes/kube-scheduler.yaml.bak`,
  is in the parent directory. The kubelet only runs manifests from the
  directory named by `staticPodPath` in `/var/lib/kubelet/config.yaml`.
- The controller manager crashes: its log says it cannot read the kubeconfig
  `/etc/kubernetes/controller-manager.config`. The file is
  `controller-manager.conf`.

```bash
mv /etc/kubernetes/kube-scheduler.yaml.bak /etc/kubernetes/manifests/kube-scheduler.yaml
sed -i 's#controller-manager.config#controller-manager.conf#' \
  /etc/kubernetes/manifests/kube-controller-manager.yaml
```

The kubelet notices the manifest changes and restarts both pods, after which
the deployment gets its ReplicaSet, pods and nodes. Fixing the manifests, not
the running containers, is what keeps the fix after a kubelet restart.
