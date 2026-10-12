`node01` needs maintenance. Prepare it so that:

- `node01` is cordoned and no new pods are scheduled on it;
- every pod in namespace `maint` leaves `node01`, except pods that belong to a
  DaemonSet;
- the standalone pod `maint/scratch` keeps running on another node, with the
  same image and command;
- deployment `maint/web` stays available throughout, and the other workloads in
  `maint` keep running.

Do not delete or change the PodDisruptionBudget `web-pdb` or the DaemonSet
`node-agent`.
