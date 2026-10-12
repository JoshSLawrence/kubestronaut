# Solution

On `node01` (`kubestronaut ssh node01`):

```bash
cat >/etc/sysctl.d/99-kubernetes.conf <<'CONF'
net.ipv4.ip_forward = 1
net.bridge.bridge-nf-call-iptables = 1
CONF
cat >/etc/modules-load.d/k8s.conf <<'CONF'
overlay
br_netfilter
CONF
modprobe overlay
modprobe br_netfilter
sysctl --system
```

Files in `/etc/sysctl.d` and `/etc/modules-load.d` are what survive a reboot;
`sysctl -w` and `modprobe` alone only change the running kernel, and writing
the files alone changes nothing until `sysctl --system` or a reboot. Load the
modules before `sysctl --system`, because the bridge setting only exists once
`br_netfilter` is loaded.

The package files are local, so install them with `dpkg`, which resolves
nothing over the network. All the `.deb` files are needed, since `ipvsadm`
depends on the libraries next to it:

```bash
dpkg -i /opt/pkgs/*.deb
```

Unpacking a package with `dpkg-deb -x` would copy the files but leave the
package uninstalled.

`crictl` reads `/etc/crictl.yaml`. The nodes run containerd, not CRI-O, so
point both endpoints at `unix:///run/containerd/containerd.sock`. Then ask the
runtime for its own name and version instead of guessing:

```bash
crictl ps
crictl version
crictl version |
  awk '/^RuntimeName:/ {n = $2} /^RuntimeVersion:/ {v = $2} END {print n, v}' \
  >/root/runtime.txt
```
