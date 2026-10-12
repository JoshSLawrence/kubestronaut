On `controlplane`, `/opt/operators/cert-manager.yaml` contains the
cert-manager operator.

1. Install it. Configure the controller so that it deletes a Certificate's
   Secret when the Certificate is deleted (the
   `--enable-certificate-owner-ref` flag).
2. Write the names of all CRDs that belong to cert-manager, one per line, to
   `/root/cert-manager-crds.txt`.
3. Write the documentation of the `spec.subject` field of the `Certificate`
   resource, as shown by `kubectl`, to `/root/subject.txt`.
4. Create a ClusterIssuer `selfsigned` that issues self-signed certificates,
   and a Certificate `web-cert` in namespace `shop` for DNS name
   `web.shop.svc` stored in Secret `web-tls`.
