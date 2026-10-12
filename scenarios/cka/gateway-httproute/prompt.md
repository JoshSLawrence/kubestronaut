The Gateway API and a Gateway controller are installed. Gateway `infra/web-gw`
(GatewayClass `traefik`) listens for HTTP on port 8000 and accepts routes only
from namespaces labelled `shared-gateway-access=true`. Its controller is
reachable on node port 30080.

In namespace `shop`, create an HTTPRoute named `api` that attaches to
`infra/web-gw` for the host `api.example.local`:

- Requests with the header `x-canary: true` go only to Service `api-v2`.
- All other requests under the path `/api` go 90% to Service `api-v1` and 10%
  to Service `api-v2`.

Both Services listen on port 8080. Do not change the Gateway.
