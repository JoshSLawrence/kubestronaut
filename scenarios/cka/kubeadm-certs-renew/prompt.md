On `controlplane`:

1. Using kubeadm, write the expiration report for all certificates managed
   by kubeadm to `/root/cert-expiry.txt`.
2. Renew the API server's serving certificate and the API server's client
   certificate for the kubelet. Do not renew any other certificate.
3. Make sure the running API server uses the renewed certificates.
