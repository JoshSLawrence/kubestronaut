A new engineer, `jane`, needs read access to pods and their logs in the
`finance` namespace, and nothing else.

On `controlplane`, jane's private key is at `/root/jane.key`.

1. Create a CertificateSigningRequest named `jane` for the user `jane` so that
   the API server issues her a client certificate. Approve it and save the
   issued certificate to `/root/jane.crt`.
2. Create a Role `pod-reader` in `finance` that allows get, list and watch on
   pods and get on pod logs. Create a RoleBinding `jane-pod-reader` that
   grants the Role to the user `jane`.

Do not give jane any access outside `finance`.
