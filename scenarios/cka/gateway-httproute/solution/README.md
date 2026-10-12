# Solution

The route is only accepted when its namespace carries the label the Gateway's
listener selects. Label the namespace instead of loosening the Gateway:

```bash
kubectl label namespace shop shared-gateway-access=true
```

The header match and the weighted split are two rules. Gateway API picks the
most specific matching rule, so the rule with the header (a header match
outranks a plain path match) takes the canary requests and everything else
under `/api` falls to the weighted rule:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: api
  namespace: shop
spec:
  parentRefs:
    - name: web-gw
      namespace: infra
  hostnames: [api.example.local]
  rules:
    - matches:
        - path: {type: PathPrefix, value: /api}
          headers:
            - {name: x-canary, value: "true"}
      backendRefs:
        - {name: api-v2, port: 8080}
    - matches:
        - path: {type: PathPrefix, value: /api}
      backendRefs:
        - {name: api-v1, port: 8080, weight: 90}
        - {name: api-v2, port: 8080, weight: 10}
```

`parentRefs` needs the `namespace` because the Gateway is not in the route's
namespace. Weights are relative (9 and 1 behave like 90 and 10); a backend
without a weight counts as 1. `kubectl describe httproute api -n shop` shows
the `Accepted` and `ResolvedRefs` conditions that explain a rejected route.
