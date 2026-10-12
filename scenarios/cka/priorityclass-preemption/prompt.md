The cluster's only node is full of low-priority batch pods, and deployment
`api` in namespace `prod` cannot be scheduled.

1. Create a PriorityClass `high-priority` whose value is exactly one less than
   the highest user-defined PriorityClass in the cluster. It must not be the
   default for pods that have no priority class.
2. Make deployment `prod/api` use it so that its pod runs.

Do not change the batch workloads in namespace `batch`, the existing
PriorityClasses, or the resources requested by `api`.
