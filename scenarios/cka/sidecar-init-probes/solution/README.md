# Solution

Edit the deployment (`kubectl -n logging edit deployment legacy-app`) so that
the pod template has:

- a volume `logs` (`emptyDir: {}`) mounted at `/var/log` in both containers.
  The app container needs it because, with a read-only root filesystem, the
  volume is the only place it can still write its log.
- an init container `log-shipper` with `restartPolicy: Always`, which makes it
  a native sidecar that starts before `app` and keeps running next to it,
  running `tail -n+1 -F /var/log/legacy-app.log`.
- a readiness probe on `app`, for example
  `exec: {command: [test, -f, /var/log/legacy-app.log]}`.
- the same `securityContext` in each container:

  ```yaml
  securityContext:
    runAsNonRoot: true
    runAsUser: 10001
    readOnlyRootFilesystem: true
    allowPrivilegeEscalation: false
  ```

Setting `runAsUser` at pod level is not enough here: the task asks for the
settings on each container, and `readOnlyRootFilesystem` and
`allowPrivilegeEscalation` exist only at container level.

Check the result with `kubectl -n logging logs deploy/legacy-app -c
log-shipper` and `kubectl -n logging get pods`.
