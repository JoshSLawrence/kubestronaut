The latest rollout of deployment `frontend` in namespace `web` is stuck.

1. Roll the deployment back to its previous working revision.
2. Scale it to 3 replicas.
3. Change its update strategy so that a rollout never has fewer than 3 ready
   pods and creates at most one extra pod at a time.
4. Every container must request 50m CPU and 64Mi memory.

Do not delete the deployment.
