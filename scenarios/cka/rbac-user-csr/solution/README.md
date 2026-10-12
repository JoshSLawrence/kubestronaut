# Solution

Build a certificate request with jane's key on `controlplane`, submit it as a
CertificateSigningRequest for the `kubernetes.io/kube-apiserver-client`
signer, approve it and save the issued certificate:

```bash
kubestronaut ssh controlplane
openssl req -new -key /root/jane.key -subj "/CN=jane" | base64 | tr -d '\n'
```

```yaml
apiVersion: certificates.k8s.io/v1
kind: CertificateSigningRequest
metadata:
  name: jane
spec:
  request: <the base64 output>
  signerName: kubernetes.io/kube-apiserver-client
  usages: [client auth]
```

```bash
kubectl apply -f csr.yaml
kubectl certificate approve jane
kubectl get csr jane -o jsonpath='{.status.certificate}' | base64 -d \
  > /root/jane.crt
```

The common name becomes the user name. Then grant the access, as a Role and a
RoleBinding so it stays inside `finance`:

```bash
kubectl -n finance apply -f - <<'YAML'
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: pod-reader
rules:
  - apiGroups: [""]
    resources: [pods]
    verbs: [get, list, watch]
  - apiGroups: [""]
    resources: [pods/log]
    verbs: [get]
YAML
kubectl -n finance create rolebinding jane-pod-reader \
  --role=pod-reader --user=jane
```

Signing a certificate yourself with the CA key under `/etc/kubernetes/pki`
would also authenticate, but it bypasses the CSR API and is not what the task
asks for. Binding jane to a ClusterRole such as `view` gives her access in
every namespace.
