# Solution

Read the three files, then apply the narrowest one that still lets the
frontend clients in:

```bash
kubestronaut ssh controlplane
cat /root/netpol/policy-*.yaml
kubectl apply -f /root/netpol/policy-b.yaml
```

- `policy-a.yaml` allows every pod in every namespace on every port, so the
  look-alike in `other` and the `debug` pod get in.
- `policy-c.yaml` allows the whole `frontend` namespace on 8080, so the
  `debug` pod gets in too.
- `policy-b.yaml` combines a namespace selector and a pod selector in one
  `from` entry (AND) and names port 8080 only. It is the only one that meets
  the requirement without allowing more.

Applying several files adds their rules together, so the result is as
permissive as the most permissive file you applied.
