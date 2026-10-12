# kubestronaut

Practice for Kubernetes certification exams on your own machine.
kubestronaut creates a [kind](https://kind.sigs.k8s.io/) cluster with a
scenario already set up, shows you the task, lets you work with real `kubectl`
and real node shells, then grades the result.

The first pack is CKA. CKAD and CKS packs plug in without engine changes; see
[docs/authoring.md](docs/authoring.md).

## Requirements

- macOS or Linux (the session lock uses `flock`, so Windows is not supported)
- Docker with at least 6 GiB of memory (a scenario uses one or two kind nodes)
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
`~/.kubestronaut`.

## Usage

| Command | Purpose |
| ------- | ------- |
| `list [--exam cka] [--domain d] [--json] [-a]` | List scenarios |
| `show <id>` | Show metadata (no prompt, no spoilers) |
| `start <id> [--k8s-version v] [--node-image img] [--force] [--keep-on-failure]` | Create cluster, run setup, print the task |
| `status` | Show the active session |
| `prompt` | Print the task again |
| `env` | Print `export KUBECONFIG=...` for `eval` |
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

`--node-image` overrides the pinned kind node image, and `--keep-on-failure`
leaves a half-built cluster up so you can inspect it. Each session directory
contains `setup.log`, which describes the injected fault: do not read it
before you are done.

Only one session runs at a time, which keeps Docker memory in check. Exam mode
(`exam ...`) is planned; see [docs/design.md](docs/design.md).

## Scenarios

| Id | Domain | Nodes |
| -- | ------ | ----- |
| `cka/rbac-dev-access` | cluster-architecture | 1 |
| `cka/networkpolicy-isolate` | services-networking | 1 |
| `cka/troubleshoot-kubelet` | troubleshooting | 2 |

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
`validation.passBeforeSolution`. Narrow it with `KS_E2E_SCENARIOS=cka/rbac-dev-access`.

Run `pre-commit install` to get the same checks on every commit.

## Documentation

- [docs/authoring.md](docs/authoring.md): add a scenario or a new pack
- [docs/design.md](docs/design.md): architecture and roadmap

## License

MIT. See [LICENSE](LICENSE).
