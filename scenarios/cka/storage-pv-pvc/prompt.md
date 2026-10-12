The logging team wants to keep application logs on the node's disk.

1. Create a PersistentVolume `pv-logs`: 1Gi, access mode ReadWriteOnce,
   hostPath `/mnt/logs`, storage class name `manual`, and a reclaim policy
   that keeps the data when its claim is deleted.
2. Create a PersistentVolumeClaim `logs` in namespace `app` that binds to
   `pv-logs`.
3. Create a pod `logger` in namespace `app` (image `busybox:1.38.0`, command
   `sh -c 'date >> /var/log/app/out.log; sleep 1d'`) with the claim mounted
   at `/var/log/app`.

The log file must end up on the node under `/mnt/logs`.
