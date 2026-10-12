# Solution

Install the chart from the local archive. `--skip-crds` leaves out the CRDs in
the chart's `crds/` directory, and `--set` gives the values:

```bash
kubestronaut ssh controlplane
helm install edge /opt/charts/traefik-41.7.1.tgz \
  --namespace ingress --create-namespace --skip-crds \
  --set deployment.replicas=2 \
  --set service.spec.type=NodePort \
  --set ports.web.nodePort=30080 \
  --set providers.kubernetesCRD.enabled=false
```

The chart takes the Service type under `service.spec.type`; `helm show values`
lists the keys. The same values in a file passed with `-f` give the same
release. Then look at
the other release and roll it back with Helm:

```bash
helm history storefront -n shop
helm rollback storefront 1 -n shop
```

`helm rollback` creates revision 3 with the contents of revision 1.
`kubectl rollout undo` would fix the deployment but leave Helm believing that
revision 2 is deployed, and the next `helm upgrade` would break it again.
`helm template ... | kubectl apply -f -` creates the objects without a
release, so Helm cannot manage or roll them back.
