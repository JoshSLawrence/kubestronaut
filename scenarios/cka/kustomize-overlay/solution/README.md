# Solution

Leave `base` alone and create the overlay next to it:

```yaml
# /opt/kustomize/metrics-server/overlays/lab/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../../base
labels:
  - pairs:
      app.kubernetes.io/part-of: monitoring
    includeSelectors: false
patches:
  - target:
      kind: Deployment
      name: metrics-server
    patch: |-
      - op: add
        path: /spec/template/spec/containers/0/args/-
        value: --kubelet-insecure-tls
```

Check it, then install it:

```bash
kubectl kustomize /opt/kustomize/metrics-server/overlays/lab
kubectl apply -k /opt/kustomize/metrics-server/overlays/lab
```

`labels` with `includeSelectors: false` labels every object without touching
selectors. The older `commonLabels` field adds the label to selectors as
well, which changes what the Service selects. A strategic merge patch that
lists the container and its full `args` works as well, as long as it keeps the
existing arguments.
