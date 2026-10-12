Pods on `node01` cannot reach anything outside the node. Prepare `node01` the
way kubeadm expects a node to be prepared:

1. IPv4 forwarding and bridged traffic filtering by iptables
   (`net.bridge.bridge-nf-call-iptables`) must be enabled now and stay enabled
   after a reboot. Put the settings in `/etc/sysctl.d/99-kubernetes.conf`.
2. The kernel modules `overlay` and `br_netfilter` must be loaded at boot.
   Use `/etc/modules-load.d/k8s.conf`.
3. Install the package `ipvsadm` from the `.deb` files in `/opt/pkgs/`. The
   node has no network access; use the system package manager.
4. `crictl` on `node01` is misconfigured. Fix it so that `crictl ps` works,
   and write the container runtime's name and version, as the runtime itself
   reports them, to `/root/runtime.txt` in the form `<name> <version>`.
