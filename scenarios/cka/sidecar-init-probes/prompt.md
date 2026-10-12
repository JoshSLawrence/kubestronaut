Deployment `legacy-app` in namespace `logging` has a container `app` that
writes its log to `/var/log/legacy-app.log` inside the container instead of to
stdout.

1. Add a native sidecar container named `log-shipper` (image
   `busybox:1.38.0`) that starts before the app and streams that file to its
   own stdout with `tail -n+1 -F`, so that
   `kubectl logs deploy/legacy-app -c log-shipper` shows the application log.
   Share `/var/log` between the two containers through an `emptyDir` volume
   named `logs`.
2. Add a readiness probe to the `app` container that succeeds only once
   `/var/log/legacy-app.log` exists.
3. Harden both containers. In the `securityContext` of each container, not of
   the pod, set `runAsNonRoot: true`, `runAsUser: 10001`,
   `readOnlyRootFilesystem: true` and `allowPrivilegeEscalation: false`.

Do not change the image or the command of the `app` container.
