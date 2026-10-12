The Traefik ingress controller is installed in namespace `traefik` with
IngressClass `traefik`, which is not the default class. It is reachable on
node ports 30080 (HTTP) and 30443 (HTTPS). Namespace `shop` has Services `api`
(port 8080) and `web` (port 80).

In namespace `shop`, create an Ingress named `shop` for the host
`shop.example.local`:

- `/api` and everything below it goes to Service `api`.
- Everything else goes to Service `web`.
- HTTPS uses the TLS Secret `shop-tls`, which you create from
  `/opt/certs/shop.crt` and `/opt/certs/shop.key` on `controlplane`.

Requests for any other host must not be routed.
