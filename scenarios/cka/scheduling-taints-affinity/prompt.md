Deployment `trainer` in namespace `ml` runs 2 replicas of a training job. Its
pods must run only on nodes that carry the label `accelerator=gpu`. Those
nodes are tainted so that ordinary workloads stay away.

Change the deployment so that both replicas are running on such a node, and
only on such a node.

Do not change any node, and do not pin the pods to a node by name.
