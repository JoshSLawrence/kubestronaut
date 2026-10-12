#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

ks_node node01 bash -s <<'SH'
set -euo pipefail
cat >/etc/sysctl.d/99-kubernetes.conf <<'CONF'
net.ipv4.ip_forward = 1
net.bridge.bridge-nf-call-iptables = 1
CONF
cat >/etc/modules-load.d/k8s.conf <<'CONF'
overlay
br_netfilter
CONF
modprobe overlay || true
modprobe br_netfilter || true
sysctl --system >/dev/null
dpkg -i /opt/pkgs/*.deb
cat >/etc/crictl.yaml <<'CONF'
runtime-endpoint: unix:///run/containerd/containerd.sock
image-endpoint: unix:///run/containerd/containerd.sock
timeout: 10
CONF
crictl version | awk '/^RuntimeName:/ {n = $2} /^RuntimeVersion:/ {v = $2} END {print n, v}' >/root/runtime.txt
SH
