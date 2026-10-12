Pod `archiver` in namespace `reports` is Pending because its
PersistentVolumeClaim `archive` does not bind.

The pod must use the existing PersistentVolume `archive-pv`, which already
holds data. Fix the claim and get the pod running with the claim mounted at
`/archive`.

Do not replace `archive-pv`, and do not change its size, access modes, storage
class, reclaim policy or path. Keep its data.
