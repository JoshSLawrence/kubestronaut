Dynamic provisioning in this cluster is not set up the way the data team
wants.

1. Create a StorageClass `local-fast` that uses the `rancher.io/local-path`
   provisioner, keeps volumes after their claim is deleted, and creates a
   volume only when a pod uses the claim.
2. Make `local-fast` the only default StorageClass.
3. PersistentVolumeClaim `cache` in namespace `data` is Pending. Do not delete
   or recreate it. Make it bind to a `local-fast` volume by running a pod
   `cache-writer` in `data` (image `busybox:1.38.0`, command
   `sh -c 'date > /cache/stamp; sleep 1d'`) with the claim mounted at
   `/cache`.
