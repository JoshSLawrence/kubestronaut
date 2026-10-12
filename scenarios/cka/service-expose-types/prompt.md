Deployment `catalog` in namespace `catalog` has 2 replicas that serve HTTP on
their container port named `http`. A load balancer implementation is installed
in the cluster.

Create three Services in namespace `catalog`:

1. `catalog`: type ClusterIP, port 80, targeting the container port by its
   name.
2. `catalog-np`: type NodePort, port 80, node port 30080.
3. `catalog-lb`: type LoadBalancer, port 80.

Only the pods of Deployment `catalog` may back these Services.
