In namespace `shop`, create a NetworkPolicy named `api-allow-web` so that only
pods labelled `app=web` in `shop` can reach pods labelled `app=api` on TCP
8080. All other ingress to `api` must be denied.

Do not change other workloads.
