Pods in the cluster cannot resolve Service names.

1. Fix cluster DNS. Do not change the cluster IP of the `kube-dns` Service.
2. Configure CoreDNS so that the name `legacy.example.internal` resolves, from
   any pod, to the Service `legacy` in namespace `apps`, and keeps resolving
   to it if that Service gets a different IP.
