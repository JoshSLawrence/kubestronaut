# Solution

Follow the request path. `kubectl -n store get endpointslices` for the Service
is empty, so look at why:

```bash
kubectl -n store get service checkout -o wide
kubectl -n store get pods --show-labels
kubectl -n store describe pods -l app=checkout
```

There are three separate faults:

- The Service selects `app=checkout-svc`, which no pod carries. The selector
  must be `app=checkout`. `tier=checkout` would also select the legacy pods.
- The Service's `targetPort` is 80, but the containers listen on 8080.
- The pods' readiness probe checks port 8081, so they never become ready and
  are left out of the endpoints.

```bash
kubectl -n store patch service checkout --type=merge \
  -p '{"spec":{"selector":{"app":"checkout"},"ports":[{"port":80,"targetPort":8080}]}}'
kubectl -n store patch deployment checkout --type=json -p \
  '[{"op":"replace","path":"/spec/template/spec/containers/0/readinessProbe/httpGet/port","value":8080}]'
```

Changing the probe rolls out new pods. A selector that matched `tier` would
send traffic to `checkout-legacy` as well.
