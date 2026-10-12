# Solution

Two faults were injected on `node01`:

1. `/var/lib/kubelet/config.yaml` points `clientCAFile` at
   `/etc/kubernetes/pki/ca.cert`, which does not exist. The file is
   `ca.crt`. `journalctl -u kubelet` shows the failure to load the client CA.
2. The kubelet unit was disabled, so it would not return after a reboot.

```bash
kubestronaut ssh node01
sed -i 's#pki/ca.cert#pki/ca.crt#' /var/lib/kubelet/config.yaml
systemctl enable --now kubelet
```

The node turns Ready after a short delay and the `pinned` pod is scheduled.
