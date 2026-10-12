Deployment `orders` in namespace `orders` keeps restarting.

1. On `controlplane`, write the last line that the previous instance of the
   container logged before it exited to `/root/orders-error.txt`. The file must
   contain only that line.
2. Fix the deployment so that it runs. The value it needs is in the ConfigMap
   `orders-config`; take it from there instead of typing it in. Do not change
   the container's image or command.
