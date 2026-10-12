# Solution

Take the snapshot of the running etcd first, with the certificates etcd uses
for its clients:

```bash
kubestronaut ssh controlplane
etcdctl --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key \
  snapshot save /opt/backup/now.db
```

Restore the earlier snapshot into a new directory. Since etcd 3.6 the offline
commands live in `etcdutl`:

```bash
etcdutl snapshot restore /opt/backup/golden.db --data-dir /var/lib/etcd-golden
```

Then point the static pod at the new directory. The `etcd-data` volume's
`hostPath.path` in `/etc/kubernetes/manifests/etcd.yaml` becomes
`/var/lib/etcd-golden`; changing `--data-dir` and the mount path together
with the host path works too. The kubelet recreates the pod, and the API
server is unavailable for a short while:

```bash
kubectl get --raw /readyz
```

The snapshot has to be taken before the restore: once etcd runs on the
restored data, the objects created since `golden.db` are gone and a snapshot
no longer holds them. Restoring into `/var/lib/etcd` fails because that
directory is not empty.

If the API server shows stale data after the restore (its watch cache can be
ahead of etcd), restart it, or restore with `--bump-revision` and
`--mark-compacted`, which make etcd's revision jump past anything the API
server has seen.
