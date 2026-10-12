On `controlplane`, `/opt/kustomize/metrics-server/base` contains the upstream
metrics-server manifests. Do not edit any file in `base`.

Create an overlay in `/opt/kustomize/metrics-server/overlays/lab` that:

- adds the argument `--kubelet-insecure-tls` to the metrics-server container,
- sets the label `app.kubernetes.io/part-of: monitoring` on every resource
  (do not change selectors),

and install metrics-server with it using `kubectl`. `kubectl top nodes` must
work afterwards.
