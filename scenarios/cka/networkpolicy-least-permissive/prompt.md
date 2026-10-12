Pods labelled `app=frontend` in namespace `frontend` must reach the pods of
deployment `backend` in namespace `backend` on TCP port 8080. Nothing else may
reach them.

Three candidate NetworkPolicies are in `/root/netpol/` on `controlplane`.
Apply the one that meets this requirement while allowing the least traffic.
Apply only one of them, do not edit the files and do not write your own
policy.
