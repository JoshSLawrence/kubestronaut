Service `checkout` in namespace `store` does not answer requests on port 80.
Deployment `checkout` is supposed to serve it.

Fix the problem so that the Service reaches all 3 `checkout` pods, and only
those. Keep the pods' labels and their readiness probe in place; change a pod
setting only if it stops the pods from becoming ready. The application must
keep listening on its current port. Leave `checkout-legacy` alone.
