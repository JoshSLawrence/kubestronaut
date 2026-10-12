# kubestronaut

Practice for Kubernetes certification exams on your own machine.
kubestronaut creates a [kind](https://kind.sigs.k8s.io/) cluster with a
scenario already set up, shows you the task, lets you work with real `kubectl`
and real node shells, then grades the result.

The first pack is CKA. CKAD and CKS packs plug in without engine changes; see
[docs/authoring.md](docs/authoring.md).

## Requirements

- macOS or Linux (the session lock uses `flock`, so Windows is not supported)
- Docker with at least 6 GiB of memory (a scenario uses one to three kind nodes)
- [mise](https://mise.jdx.dev/) to install the pinned toolchain

`mise install` provides Go, kind and kubectl at the versions in `mise.toml`.
`kubestronaut doctor` checks that everything works.

## Quickstart

```bash
mise install
mise run build

./bin/kubestronaut list
./bin/kubestronaut start troubleshoot-kubelet
eval "$(./bin/kubestronaut env)"
kubectl get nodes
./bin/kubestronaut ssh node01
./bin/kubestronaut grade
./bin/kubestronaut stop
```

`start` prints the task and an `export KUBECONFIG=...` line. kubestronaut never
reads or writes `~/.kube/config`; it keeps its own kubeconfig under
`~/.kubestronaut`. `kubestronaut env` and `shell` also set `KUBECACHEDIR` to
`~/.kubestronaut/cache`, so kubectl's discovery cache stays out of `~/.kube`.

## Usage

| Command | Purpose |
| ------- | ------- |
| `list [--exam cka] [--domain d] [--json] [-a]` | List scenarios |
| `show <id>` | Show metadata (no prompt, no spoilers) |
| `start <id> [--k8s-version v] [--node-image img] [--force] [--keep-on-failure]` | Create cluster, run setup, print the task |
| `status` | Show the active session |
| `prompt` | Print the task again |
| `env` | Print `export KUBECONFIG=...` and `KUBECACHEDIR` for `eval` |
| `shell` | Start `$SHELL` with `KUBECONFIG` set |
| `ssh <node> [-- cmd...]` | Shell or command on `controlplane`, `node01`, ... |
| `hint [--all]` | Show the next hint (counted in the grade) |
| `grade [--json] [-v]` | Run the checks and print the score |
| `solution [--yes]` | Show the reference solution (flagged in the grade) |
| `reset [--yes]` | Recreate the cluster and rerun setup |
| `stop [--all]` | Delete the cluster; `--all` deletes every `ks-*` cluster |
| `validate [<id>...\|--all] [--e2e]` | Static checks, or full end-to-end proof |
| `doctor [--fix]` | Check docker, kind, kubectl, memory, orphan clusters |
| `version` | Version and detected tool versions |

A scenario id looks like `cka/troubleshoot-kubelet`. The name alone is accepted
when it is unique.

Global flags:

- `--scenarios-dir <dir>` (env `KUBESTRONAUT_SCENARIOS`) adds scenarios to, or
  overrides, the built-in packs.
- `--home <dir>` (env `KUBESTRONAUT_HOME`, default `~/.kubestronaut`) is where
  state lives.
- `-v` prints setup logs and check timings.

Exit codes: 0 success or grade passed, 1 grade or validation did not pass,
2 usage or runtime error, 3 no active session.

### Node image

The stock kind node image has no editor, but the exam expects you to edit
manifests and kubelet config on the node. So the first `start` for each
Kubernetes version builds `kubestronaut/node:<version>-<hash>` locally on top
of the pinned kind image. It adds `vim`, `nano`, `less`, DNS and process
tools, vim settings for YAML (two-space indent, no tabs), and the `k` alias
with completion for kubectl. It also adds `etcdctl` and `etcdutl` in the etcd
release that matches the cluster's etcd, Helm 3.22.0, the `bridge` CNI plugin
(in `/opt/cni/bin`) and the `ipvsadm` packages as `.deb` files in `/opt/pkgs`.
Every download except the `ipvsadm` packages is checked against a pinned
sha256; apt verifies those by signature. The build usually takes
under a minute and needs internet once; later starts reuse it. The hash
covers the Dockerfile (`internal/backend/kind/nodeimage/Dockerfile`), the base
image and the etcd pin, so changing any of them rebuilds. `kubestronaut
doctor` says whether it is built yet and lists old images you can remove.

`--node-image` uses your own image instead and skips the build; it will lack
the editors unless you add them. `reset` keeps the same image.
`--keep-on-failure` leaves a half-built cluster up so you can inspect it.
Each session directory contains `setup.log`, which describes the injected
fault: do not read it before you are done.

Only one session runs at a time, which keeps Docker memory in check. Exam mode
(`exam ...`) is planned; see [docs/design.md](docs/design.md).

## Scenarios

| Id                                   | Domain               | Nodes | Time |
| ------------------------------------ | -------------------- | ----- | ---- |
| `cka/cni-install-calico`             | cluster-architecture | 3     | 15m  |
| `cka/configmap-secret-env`           | workloads-scheduling | 1     | 10m  |
| `cka/crd-operator-cert-manager`      | cluster-architecture | 2     | 15m  |
| `cka/deployment-rollout-rollback`    | workloads-scheduling | 1     | 8m   |
| `cka/etcd-backup-restore`            | cluster-architecture | 1     | 15m  |
| `cka/gateway-httproute`              | services-networking  | 2     | 12m  |
| `cka/gateway-migrate-ingress-tls`    | services-networking  | 2     | 15m  |
| `cka/helm-install-rollback`          | cluster-architecture | 1     | 12m  |
| `cka/hpa-autoscale`                  | workloads-scheduling | 2     | 10m  |
| `cka/ingress-traefik`                | services-networking  | 2     | 12m  |
| `cka/kubeadm-certs-renew`            | cluster-architecture | 1     | 10m  |
| `cka/kustomize-overlay`              | cluster-architecture | 2     | 12m  |
| `cka/monitor-resource-usage`         | troubleshooting      | 2     | 8m   |
| `cka/networkpolicy-isolate`          | services-networking  | 1     | 10m  |
| `cka/networkpolicy-least-permissive` | services-networking  | 1     | 8m   |
| `cka/node-maintenance-drain`         | cluster-architecture | 3     | 10m  |
| `cka/priorityclass-preemption`       | workloads-scheduling | 1     | 10m  |
| `cka/quota-limitrange`               | workloads-scheduling | 1     | 10m  |
| `cka/rbac-dev-access`                | cluster-architecture | 1     | 8m   |
| `cka/rbac-user-csr`                  | cluster-architecture | 1     | 12m  |
| `cka/scheduling-taints-affinity`     | workloads-scheduling | 3     | 10m  |
| `cka/sidecar-init-probes`            | workloads-scheduling | 1     | 12m  |
| `cka/service-expose-types`           | services-networking  | 2     | 12m  |
| `cka/storage-pv-pvc`                 | storage              | 1     | 8m   |
| `cka/storage-pvc-pending`            | storage              | 1     | 10m  |
| `cka/storageclass-dynamic`           | storage              | 1     | 10m  |
| `cka/troubleshoot-apiserver`         | troubleshooting      | 1     | 15m  |
| `cka/troubleshoot-control-plane`     | troubleshooting      | 1     | 12m  |
| `cka/troubleshoot-coredns`           | troubleshooting      | 1     | 12m  |
| `cka/troubleshoot-crashloop-logs`    | troubleshooting      | 1     | 8m   |
| `cka/troubleshoot-kube-proxy`        | troubleshooting      | 2     | 12m  |
| `cka/troubleshoot-kubelet`           | troubleshooting      | 2     | 10m  |
| `cka/troubleshoot-service-endpoints` | troubleshooting      | 1     | 10m  |

## Development

```bash
mise run lint      # golangci-lint, markdownlint, shellcheck/shfmt, actionlint
mise run test      # unit tests with the race detector
mise run validate  # static validation of every scenario
mise run ci        # all of the above
mise run e2e       # real clusters; takes several minutes
```

`mise run e2e` runs `kubestronaut validate --e2e` for each scenario: it starts
a cluster, requires every check to fail, runs the reference solution, and
requires a 100% grade. Checks that legitimately pass before the solution (for
example "must stay denied" checks) are listed in
`validation.passBeforeSolution`. Narrow the run with
`KS_E2E_SCENARIOS=cka/rbac-dev-access`.

Run `pre-commit install` to get the same checks on every commit.

## Documentation

- [docs/authoring.md](docs/authoring.md): add a scenario or a new pack
- [docs/design.md](docs/design.md): architecture and roadmap

## License

MIT. See [LICENSE](LICENSE).
