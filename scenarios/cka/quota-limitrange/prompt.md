Namespace `team-a` has no resource governance yet. Deployment `web` (3
replicas) runs there without any resource requests or limits.

1. Create a LimitRange `container-defaults` in `team-a` so that containers
   without resources get a CPU request of 100m, a memory request of 64Mi, a
   CPU limit of 200m and a memory limit of 128Mi. No container may set a CPU
   limit above 500m.
2. Create a ResourceQuota `team-a-quota` in `team-a` that limits the namespace
   to 5 pods, 1 CPU of requests in total and 1Gi of memory limits in total.
3. All three `web` pods must be running and must have picked up the defaults.

Do not add resources to the `web` deployment yourself.
