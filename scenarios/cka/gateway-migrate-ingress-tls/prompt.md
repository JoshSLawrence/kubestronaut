Ingress `web/web` (IngressClass `traefik`) serves `https://web.example.local`
with the TLS Secret `web-tls`. The Gateway API is installed, and the same
Traefik controller implements GatewayClass `traefik`. It is reachable on node
port 30443, which forwards to its HTTPS entry point on port 8443.

Migrate the application to the Gateway API:

1. In namespace `web`, create Gateway `web-gateway` with GatewayClass
   `traefik` and an HTTPS listener on port 8443 for the hostname
   `web.example.local` that terminates TLS with the Secret `web-tls`.
2. Create HTTPRoute `web-route` in `web` that attaches to the Gateway and
   sends all traffic for that host to Service `web` port 80.
3. Delete the Ingress once the route works.
