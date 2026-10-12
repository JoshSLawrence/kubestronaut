# Solution

CPU utilisation is measured against the container's CPU request, and the
deployment has none, so the autoscaler reports `FailedGetResourceMetric`
(`kubectl describe hpa php-apache`). Add a request:

```bash
kubectl -n autoscale set resources deployment/php-apache --requests=cpu=100m
```

`kubectl autoscale` cannot set the scale-down delay, so write the
autoscaling/v2 object:

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: php-apache
  namespace: autoscale
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: php-apache
  minReplicas: 1
  maxReplicas: 4
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 50
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 30
```

`kubectl autoscale deployment php-apache --cpu-percent=50 --min=1 --max=4`
followed by `kubectl edit hpa php-apache` to add `behavior` gives the same
result. Within a minute `kubectl get hpa` shows a CPU percentage instead of
`<unknown>`.
