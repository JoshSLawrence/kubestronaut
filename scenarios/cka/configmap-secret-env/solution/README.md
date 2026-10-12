# Solution

The file is on the control plane, so create the ConfigMap from there
(`kubestronaut ssh controlplane`). `kubectl create configmap` has no flag for
`immutable`, so add the field to the generated manifest:

```bash
kubectl -n config create configmap app-config \
  --from-file=/opt/config/app.properties --dry-run=client -o yaml |
  sed '/^kind: ConfigMap/a immutable: true' |
  kubectl apply -f -
```

Create the Secret, quoting the password so the shell leaves the `!` alone:

```bash
kubectl -n config create secret generic db-creds \
  --from-literal=username=app --from-literal=password='S3cure!pass'
```

Then edit the deployment with `kubectl -n config edit deployment api` and add
to the container, plus a matching `volumes` entry:

```yaml
env:
  - name: DB_USER
    valueFrom: {secretKeyRef: {name: db-creds, key: username}}
  - name: DB_PASSWORD
    valueFrom: {secretKeyRef: {name: db-creds, key: password}}
volumeMounts:
  - name: config
    mountPath: /etc/app/app.properties
    subPath: app.properties
    readOnly: true
```

```yaml
volumes:
  - name: config
    configMap:
      name: app-config
```

Mounting the whole ConfigMap at `/etc/app` works as well, since the key is
already called `app.properties`. The edit changes the pod template, so the
pods are replaced; the old ones never see the new configuration.
