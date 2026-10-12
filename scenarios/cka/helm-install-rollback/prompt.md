Helm is installed on `controlplane`.

1. Install the Traefik chart from `/opt/charts/traefik-41.7.1.tgz` as release
   `edge` in a new namespace `ingress`, with these settings: 2 replicas; a
   NodePort Service with the `web` port on node port 30080; the Kubernetes CRD
   provider disabled. Do not install the chart's CRDs.
2. Release `storefront` in namespace `shop` was just upgraded and is broken.
   Roll it back with Helm to the last working revision.
