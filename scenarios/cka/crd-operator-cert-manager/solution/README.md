# Solution

Apply the manifest from the node. A plain `kubectl apply -f`, `kubectl create
-f` and `--server-side` all work:

```bash
kubestronaut ssh controlplane
kubectl apply --server-side -f /opt/operators/cert-manager.yaml
```

Add the flag to the controller's arguments, either in the file before
applying or on the deployment afterwards (`kubectl -n cert-manager edit
deployment cert-manager`); only the `cert-manager` deployment is the
controller:

```yaml
args:
  - --v=2
  - --enable-certificate-owner-ref
```

The CRDs of cert-manager are the ones in the `cert-manager.io` and
`acme.cert-manager.io` groups, not the unrelated `widgets.example.com`:

```bash
kubectl get crd -o name | sed 's#.*/##' | grep 'cert-manager\.io$' \
  > /root/cert-manager-crds.txt
kubectl explain certificate.spec.subject > /root/subject.txt
```

Wait for the three deployments, then create the objects. A `ClusterIssuer`
with `spec.selfSigned: {}` issues certificates signed by their own key:

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: selfsigned
spec:
  selfSigned: {}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: web-cert
  namespace: shop
spec:
  secretName: web-tls
  dnsNames: [web.shop.svc]
  issuerRef:
    kind: ClusterIssuer
    name: selfsigned
```

If the webhook is not ready yet, the apply fails with a connection error:
wait and repeat. A Secret made by hand with `openssl` carries no
cert-manager annotations, and the Certificate would not become ready.
