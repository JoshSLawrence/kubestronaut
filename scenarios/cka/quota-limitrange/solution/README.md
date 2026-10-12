# Solution

```bash
kubectl apply -f - <<'YAML'
apiVersion: v1
kind: LimitRange
metadata:
  name: container-defaults
  namespace: team-a
spec:
  limits:
    - type: Container
      defaultRequest: {cpu: 100m, memory: 64Mi}
      default: {cpu: 200m, memory: 128Mi}
      max: {cpu: 500m}
YAML

kubectl -n team-a create quota team-a-quota \
  --hard=pods=5,requests.cpu=1,limits.memory=1Gi

kubectl -n team-a rollout restart deployment/web
```

In a LimitRange, `default` is the limit and `defaultRequest` is the request;
`max` caps the limit a container may set. A LimitRange only acts when a pod is
created, so the running pods keep having no resources until the rollout
replaces them. With a quota on requests and limits, pods that declare neither
are rejected, which is why the LimitRange has to exist first.
