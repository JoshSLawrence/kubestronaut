# Adds the tools a CKA/CKAD/CKS exam node has but the minimal kind node image
# lacks: you are expected to edit manifests and kubelet config in place.
ARG BASE
FROM ${BASE}

# Lets `kubestronaut doctor` find images left behind by older versions.
LABEL io.kubestronaut.node="1"

RUN apt-get update \
  && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    bash-completion \
    dnsutils \
    etcd-client \
    less \
    nano \
    net-tools \
    procps \
    vim \
  && rm -rf /var/lib/apt/lists/*

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
