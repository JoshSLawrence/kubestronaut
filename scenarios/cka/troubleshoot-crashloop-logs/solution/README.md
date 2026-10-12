# Solution

The log of the instance that crashed is available with `--previous` while the
next instance is running. Between restarts `kubectl logs` without the flag
shows the instance that just exited. Either way the last line is the error
message:

```bash
kubectl -n orders logs deployment/orders --previous | tail -n1 |
  kubestronaut ssh controlplane -- sh -c 'cat > /root/orders-error.txt'
```

The message says `DB_HOST` is not set, and the host is in the ConfigMap under
the key `database-host`. Add the variable to the container so that it comes
from there:

```yaml
env:
  - name: DB_HOST
    valueFrom:
      configMapKeyRef:
        name: orders-config
        key: database-host
```

`kubectl -n orders edit deployment orders` is enough. Typing the host as a
literal `value` would run, but it copies the value instead of reading it from
the ConfigMap. Replacing the command with `sleep` hides the error instead of
fixing it.
