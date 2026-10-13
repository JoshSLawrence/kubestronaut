# Adds the tools a CKA/CKAD/CKS exam node has but the minimal kind node image
# lacks: you are expected to edit manifests and kubelet config in place.
ARG BASE
FROM ${BASE}

# The etcd release matching the cluster's etcd, chosen per Kubernetes version
# by internal/backend/kind/images.go. The checksums travel with it.
ARG ETCD_VERSION
ARG ETCD_SHA256_AMD64
ARG ETCD_SHA256_ARM64

# Lets `kubestronaut doctor` find images left behind by older versions.
LABEL io.kubestronaut.node="1"

RUN apt-get update \
  && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    apt-utils \
    bash-completion \
    dnsutils \
    less \
    nano \
    net-tools \
    procps \
    vim \
  # ipvsadm and the libraries it needs, kept as files so a scenario can
  # install it with dpkg while the node has no network. apt checks the
  # signatures of what it downloads.
  && mkdir -p /opt/pkgs/partial \
  && apt-get -o Dir::Cache::archives=/opt/pkgs -o APT::Sandbox::User=root \
    install -y --download-only --no-install-recommends ipvsadm \
  && rm -rf /opt/pkgs/partial /opt/pkgs/lock /var/lib/apt/lists/*

# etcdctl and etcdutl from the etcd release that matches the cluster: etcd 3.6
# moved snapshot restore from etcdctl to etcdutl, and the Debian package
# ships neither the right version nor etcdutl.
RUN set -eux; \
  : "${ETCD_VERSION:?pass --build-arg ETCD_VERSION}"; \
  arch=$(dpkg --print-architecture); \
  case "$arch" in \
    amd64) sum="$ETCD_SHA256_AMD64" ;; \
    arm64) sum="$ETCD_SHA256_ARM64" ;; \
    *) echo "unsupported architecture $arch" >&2; exit 1 ;; \
  esac; \
  dir="etcd-v${ETCD_VERSION}-linux-${arch}"; \
  curl -fsSL -o /tmp/etcd.tar.gz \
    "https://github.com/etcd-io/etcd/releases/download/v${ETCD_VERSION}/${dir}.tar.gz"; \
  echo "${sum}  /tmp/etcd.tar.gz" | sha256sum -c -; \
  tar -xzf /tmp/etcd.tar.gz -C /usr/local/bin --strip-components=1 \
    "${dir}/etcdctl" "${dir}/etcdutl"; \
  rm /tmp/etcd.tar.gz; \
  etcdctl version; etcdutl version

# Helm for the exam's chart tasks. The legacy builder does not set
# TARGETARCH, so dpkg names the architecture.
RUN set -eux; \
  arch=$(dpkg --print-architecture); \
  case "$arch" in \
    amd64) sum=1e4ab49e429626cf6c6958d914248b78c9730803c2751b87627e171dc800e7bb ;; \
    arm64) sum=f14e804dfee240f55525b667488fe9adca349e63e00c9af634c0beb1421ac310 ;; \
    *) echo "unsupported architecture $arch" >&2; exit 1 ;; \
  esac; \
  curl -fsSL -o /tmp/helm.tar.gz "https://get.helm.sh/helm-v3.22.0-linux-${arch}.tar.gz"; \
  echo "${sum}  /tmp/helm.tar.gz" | sha256sum -c -; \
  tar -xzf /tmp/helm.tar.gz -C /usr/local/bin --strip-components=1 "linux-${arch}/helm"; \
  rm /tmp/helm.tar.gz; \
  helm version --short

# The bridge CNI plugin, which kind nodes lack and Flannel needs. Only that
# one binary is added to /opt/cni/bin; kindnet does not use it.
RUN set -eux; \
  arch=$(dpkg --print-architecture); \
  case "$arch" in \
    amd64) sum=b98f74a0f8522f0a83867178729c1aa70f2158f90c45a2ca8fa791db1c76b303 ;; \
    arm64) sum=56171987d3947707c3563db2f4001bccaf50fd63468611b9f3cbecb1375ee7ec ;; \
    *) echo "unsupported architecture $arch" >&2; exit 1 ;; \
  esac; \
  curl -fsSL -o /tmp/cni.tgz \
    "https://github.com/containernetworking/plugins/releases/download/v1.9.1/cni-plugins-linux-${arch}-v1.9.1.tgz"; \
  echo "${sum}  /tmp/cni.tgz" | sha256sum -c -; \
  tar -xzf /tmp/cni.tgz -C /opt/cni/bin ./bridge; \
  rm /tmp/cni.tgz; \
  test -x /opt/cni/bin/bridge

# kubeadm, kubelet and kubectl of the next minor as a local, offline apt
# repository, so an upgrade lab runs the real `apt-get install` flow. Only
# images whose Kubernetes version has a published next minor get it
# (internal/backend/kind/images.go); for the others the step does nothing.
# Each deb is pinned by sha256, taken from the signed Packages index of
# pkgs.k8s.io. The source is not enabled here: a scenario installs
# /opt/kubernetes-apt/kubernetes.list, so the other labs never see it.
ARG K8S_APT_MINOR
ARG K8S_APT_VERSION
ARG K8S_APT_SHA256_AMD64
ARG K8S_APT_SHA256_ARM64
RUN set -eux; \
  if [ -z "${K8S_APT_MINOR}" ]; then exit 0; fi; \
  arch=$(dpkg --print-architecture); \
  case "$arch" in \
    amd64) sums="$K8S_APT_SHA256_AMD64" ;; \
    arm64) sums="$K8S_APT_SHA256_ARM64" ;; \
    *) echo "unsupported architecture $arch" >&2; exit 1 ;; \
  esac; \
  repo="/opt/kubernetes-apt/${K8S_APT_MINOR}"; \
  mkdir -p "$repo"; \
  for entry in $sums; do \
    pkg="${entry%%=*}"; sum="${entry#*=}"; \
    deb="${pkg}_${K8S_APT_VERSION}_${arch}.deb"; \
    curl -fsSL -o "$repo/$deb" \
      "https://pkgs.k8s.io/core:/stable:/${K8S_APT_MINOR}/deb/${arch}/${deb}"; \
    echo "${sum}  $repo/$deb" | sha256sum -c -; \
  done; \
  cd "$repo"; \
  apt-ftparchive packages . > /tmp/Packages; mv /tmp/Packages Packages; \
  apt-ftparchive release . > /tmp/Release; mv /tmp/Release Release; \
  printf 'deb [trusted=yes] file:%s ./\n' "$repo" > /opt/kubernetes-apt/kubernetes.list; \
  mkdir -p /var/lib/apt/lists/partial; \
  shim=$(mktemp -d); mkdir "$shim/DEBIAN"; \
  printf '%s\n' \
    'Package: kind-cni-plugins' \
    'Version: 1.9.1-0' \
    'Architecture: all' \
    'Maintainer: kubestronaut' \
    'Provides: kubernetes-cni (= 1.9.1)' \
    'Description: CNI plugins that the kind node image ships outside dpkg' \
    ' Satisfies the kubelet package dependency without replacing /opt/cni/bin.' \
    > "$shim/DEBIAN/control"; \
  dpkg-deb --build "$shim" /tmp/kind-cni-plugins.deb; \
  dpkg -i /tmp/kind-cni-plugins.deb; \
  rm -rf "$shim" /tmp/kind-cni-plugins.deb

# The settings exam guides recommend so YAML pasted or typed in vim keeps
# two-space indentation and never gets tabs.
RUN printf '%s\n' \
    'set expandtab' \
    'set tabstop=2' \
    'set shiftwidth=2' \
    'set autoindent' \
    > /etc/vim/vimrc.local

# Match the exam's `k` alias and completion for kubectl on the node. kind
# starts control-plane nodes with KUBECONFIG=/etc/kubernetes/admin.conf, so
# `k` works there; workers have no admin kubeconfig, as on an exam node.
RUN kubectl completion bash > /etc/bash_completion.d/kubectl \
  && printf '%s\n' \
    'alias k=kubectl' \
    'complete -o default -F __start_kubectl k' \
    >> /etc/bash.bashrc
