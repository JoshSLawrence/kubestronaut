etcd runs as a static pod on `controlplane`. Its certificates are in
`/etc/kubernetes/pki/etcd`.

1. Take a snapshot of the current etcd data and save it to
   `/opt/backup/now.db`.
2. A snapshot taken earlier is at `/opt/backup/golden.db`. Restore it into
   the new data directory `/var/lib/etcd-golden` and make the running etcd
   use that directory.

Do this in that order. The cluster must be healthy when you are done.
