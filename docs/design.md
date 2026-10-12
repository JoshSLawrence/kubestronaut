# kubestronaut design

Status: draft v1 (2026-10-11). Module: `github.com/JoshSLawrence/kubestronaut`.

kubestronaut is a local harness for Kubernetes certification practice. It
creates a kind cluster with a scenario already set up, shows the task prompt,
lets you work with real `kubectl` and node shells, then grades the result.
The first pack is CKA. CKAD and CKS are later packs that need no engine
changes.

Environment it was checked against: go 1.27.2, kind v0.33.0, kubectl 1.37,
Docker 29.8.2, mise 2026.10.7, macOS arm64.

## 1. CLI and UX

### 1.1 Commands

| Command | Purpose |
| ------- | ------- |
| `list [--exam cka] [--domain d] [--json]` | List scenarios |
| `show <id>` | Show metadata (no prompt, no spoilers) |
| `start <id> [--k8s-version v] [--force]` | Create cluster, run setup, print prompt |
| `status` | Show active session, time left, contexts |
| `prompt` | Print the prompt again |
| `env` | Print `export KUBECONFIG=...` (for `eval`) |
| `shell` | Start `$SHELL` with KUBECONFIG set |
| `ssh <node> [-- cmd...]` | Shell or command on a node (`controlplane`, `node01`) |
| `hint [--all]` | Show the next hint (counted) |
| `grade [--json] [-v]` | Run checks, print score breakdown |
| `solution [--yes]` | Show the reference solution (flagged in grade) |
| `reset [--yes]` | Recreate cluster and re-run setup |
| `stop [--all]` | Delete session clusters; `--all` deletes every `ks-*` |
| `validate [<id>...\|--all] [--e2e]` | Static validation, or e2e proof |
| `doctor` | Check docker/kind/kubectl, memory, orphan clusters |
| `exam list` | List exam definitions |
| `exam start <exam>` | Start a timed multi-task exam |
| `exam task [n]` | Show task n (prompt and context to use) |
| `exam status` | Time left, task list, flags |
| `exam submit [--yes]` | Grade all tasks, weighted total, pass/fail |
| `version` | Version plus detected tool versions |

Scenario IDs look like `cka/troubleshoot-kubelet`. A unique suffix such as
`troubleshoot-kubelet` is also accepted.

Exit codes:

- 0: success, or the grade passed
- 1: the grade did not pass
- 2: usage or runtime error
- 3: no active session

Global flags: `--scenarios-dir <dir>` (env `KUBESTRONAUT_SCENARIOS`), which
adds to or overrides the embedded packs; `--home <dir>` (env
`KUBESTRONAUT_HOME`, default `~/.kubestronaut`); `-v` for verbose logs.

### 1.2 Example session

```text
$ kubestronaut list --exam cka
ID                              DOMAIN                SIZE  TIME
cka/networkpolicy-isolate       services-networking   1     10m
cka/rbac-dev-access             cluster-architecture  1     8m
cka/troubleshoot-kubelet        troubleshooting       2     10m

$ kubestronaut start troubleshoot-kubelet
==> Creating cluster ks-cka-troubleshoot-kubelet (v1.36.4, 2 nodes)  48s
==> Running setup (3 steps)                                         71s
==> Ready.

  Context: k8s-kubelet      Suggested time: 10m      Weight: 7%

  Node node01 is NotReady and the workload pinned to it is not running.
  Find and fix the cause. The fix must survive a node reboot.

  Use `kubestronaut ssh node01` to reach the node.

  export KUBECONFIG=/Users/josh/.kubestronaut/kubeconfig

$ eval "$(kubestronaut env)"
$ kubectl get nodes
NAME                                        STATUS     ROLES ...
ks-cka-troubleshoot-kubelet-control-plane   Ready      control-plane
ks-cka-troubleshoot-kubelet-worker          NotReady   <none>
$ kubestronaut ssh node01
root@ks-cka-troubleshoot-kubelet-worker:/# systemctl status kubelet
...
$ kubestronaut grade
  [PASS] 2/2  kubelet-enabled   kubelet is enabled on node01
  [FAIL] 0/3  node01-ready      node01 is Ready
  [FAIL] 0/2  pinned-ready      deployment ops/pinned is available
  Score: 2/7 (28%)  -- not passed (needs 100%)   elapsed 6m12s
$ kubestronaut hint
Hint 1/2: Compare the paths in /var/lib/kubelet/config.yaml with
the files in /etc/kubernetes/pki.
...
$ kubestronaut grade
  Score: 7/7 (100%)  -- PASSED  elapsed 9m03s  hints used: 1
$ kubestronaut stop
==> Deleted cluster ks-cka-troubleshoot-kubelet
```

Exam mode:

```text
$ kubestronaut exam start cka/mock-1
==> 2 clusters (k8s-c1: 2 nodes, k8s-c2: 2 nodes), 6 tasks, 120m
$ kubestronaut exam task 3
  Task 3/6 (weight 7%)  Context: k8s-c2   Time left: 1h41m
  Run first: kubectl config use-context k8s-c2
  ...prompt...
$ kubestronaut exam submit
  Task 1  cka/rbac-dev-access        4/4   100%  x 4% = 4.0
  Task 3  cka/troubleshoot-kubelet   2/7    28%  x 7% = 2.0
  ...
  Total 71.4% (pass mark 66%) -- PASSED  finished 1h52m
```

## 2. Repo layout

```text
kubestronaut/
├── .github/
│   └── workflows/
│       ├── ci.yml
│       └── e2e.yml
├── .gitignore
├── .golangci.yml
├── .markdownlint-cli2.yaml
├── .pre-commit-config.yaml
├── LICENSE
├── README.md
├── cmd/
│   └── kubestronaut/
│       └── main.go
├── docs/
│   ├── authoring.md
│   └── design.md
├── go.mod
├── go.sum
├── internal/
│   ├── backend/
│   │   ├── backend.go
│   │   ├── fake/
│   │   │   └── fake.go
│   │   └── kind/
│   │       ├── config.go
│   │       ├── config_test.go
│   │       ├── images.go
│   │       ├── kind.go
│   │       ├── kind_test.go
│   │       ├── nodeimage/
│   │       │   └── Dockerfile
│   │       ├── nodeimage.go
│   │       └── nodeimage_test.go
│   ├── check/
│   │   ├── cani.go
│   │   ├── check.go
│   │   ├── check_test.go
│   │   ├── connectivity.go
│   │   ├── exec.go
│   │   ├── jsonpath.go
│   │   ├── ready.go
│   │   ├── registry.go
│   │   ├── resource.go
│   │   └── script.go
│   ├── cli/
│   │   ├── exam.go
│   │   ├── grade.go
│   │   ├── hint.go
│   │   ├── lifecycle.go
│   │   ├── list.go
│   │   ├── node.go
│   │   ├── root.go
│   │   └── validate.go
│   ├── exam/
│   │   ├── exam.go
│   │   └── exam_test.go
│   ├── execx/
│   │   ├── execx.go
│   │   └── fake.go
│   ├── grade/
│   │   ├── grade.go
│   │   └── grade_test.go
│   ├── kube/
│   │   ├── fake.go
│   │   ├── kube.go
│   │   └── kubectl.go
│   ├── kubeconfig/
│   │   ├── kubeconfig.go
│   │   └── kubeconfig_test.go
│   ├── lab/
│   │   ├── lab.go
│   │   └── lab_test.go
│   ├── scenario/
│   │   ├── load.go
│   │   ├── load_test.go
│   │   ├── template.go
│   │   ├── types.go
│   │   └── validate.go
│   ├── session/
│   │   ├── session.go
│   │   └── session_test.go
│   ├── setup/
│   │   ├── lib.sh
│   │   ├── setup.go
│   │   └── setup_test.go
│   ├── ui/
│   │   └── ui.go
│   └── validate/
│       └── e2e.go
├── mise.toml
├── scenarios/
│   ├── cka/
│   │   ├── exams/
│   │   │   └── mock-1.yaml
│   │   ├── networkpolicy-isolate/
│   │   │   ├── prompt.md
│   │   │   ├── scenario.yaml
│   │   │   ├── setup/
│   │   │   │   └── workloads.yaml
│   │   │   └── solution/
│   │   │       ├── README.md
│   │   │       └── solve.sh
│   │   ├── pack.yaml
│   │   ├── rbac-dev-access/
│   │   │   └── ...
│   │   └── troubleshoot-kubelet/
│   │       ├── checks/
│   │       ├── prompt.md
│   │       ├── scenario.yaml
│   │       ├── setup/
│   │       │   ├── break-kubelet.sh
│   │       │   └── pinned.yaml
│   │       └── solution/
│   │           ├── README.md
│   │           └── solve.sh
│   ├── embed.go
│   └── testdata/
│       └── ...
└── test/
    └── e2e/
        └── e2e_test.go
```

`scenarios/embed.go` is `package scenarios` with `//go:embed *` and
`var FS embed.FS`. go:embed cannot reach parent directories, which is why the
file sits here. Adding a pack means adding a directory; no engine code and
no embed edit. Test fixtures go in
`internal/scenario/testdata`, not in the embedded tree. (The
`scenarios/testdata` entry in the tree above is a placeholder; drop it.)

## 3. Go architecture

### 3.1 Dependencies (pinned in go.mod)

- `github.com/spf13/cobra v1.10.2`: CLI.
- `sigs.k8s.io/yaml v1.6.0`: YAML to JSON, then strict decode with
  `DisallowUnknownFields`.
- `k8s.io/client-go v0.37.1`, package `util/jsonpath` only. It imports just
  the standard library and a forked `text/template`, so the binary stays small
  and the semantics match `kubectl -o jsonpath` exactly.
- Tests use the standard library only (no testify).

### 3.2 client-go clientset or kubectl?

Decision: shell out to `kubectl --kubeconfig <admin> ... -o json` and
evaluate in-process.

- kubectl is already a hard requirement, because the user works with it.
- The clientset would bring in apimachinery and api (a large dep tree and
  slow builds) for little gain.
- Grading speed is not critical (about 10 to 30 calls per grade).
- Node, auth and exec operations (`auth can-i`, `exec`) are one-liners with
  kubectl.
- Everything goes through a `kube.Client` interface, so tests use an
  in-memory fake. Moving to the clientset later is a local change.

### 3.3 Packages and key types

```go
// internal/execx: every external process goes through here so tests can
// fake docker/kind/kubectl without the binaries installed.
type Cmd struct {
    Name  string
    Args  []string
    Env   []string
    Dir   string
    Stdin io.Reader
}
type Result struct {
    Stdout, Stderr []byte
    ExitCode       int
}
type Runner interface {
    Run(ctx context.Context, c Cmd) (Result, error) // err only if not run
    Interactive(ctx context.Context, c Cmd) error   // tty passthrough
}
```

```go
// internal/backend
type NodeRole string // "control-plane" | "worker"

type ClusterSpec struct {
    Name              string // ks-<slug>, <= 40 chars
    KubernetesVersion string // "v1.36.4" -> pinned image by digest
    NodeImage         string // optional raw override
    ControlPlanes     int    // 1 or 3; 0 means 1
    Workers           int
    Kubeconfig        string // path kind writes the admin config to
}
type Node struct {
    Name  string   // backend/k8s node name, e.g. ks-x-worker
    Alias string   // controlplane, controlplane02, node01 ...
    Role  NodeRole
}
type Backend interface {
    Name() string
    // Prepare builds or fetches what Create needs (the node image for kind);
    // notify is called before slow work so the user knows why they wait.
    Prepare(ctx context.Context, s ClusterSpec, notify func(string)) error
    Create(ctx context.Context, s ClusterSpec) error
    Delete(ctx context.Context, name string) error
    List(ctx context.Context) ([]string, error) // only ks-* clusters
    Nodes(ctx context.Context, cluster string) ([]Node, error)
    NodeExec(ctx context.Context, cluster, alias string,
        stdin io.Reader, argv ...string) (execx.Result, error)
    NodeShell(ctx context.Context, cluster, alias string) error
    LoadImages(ctx context.Context, cluster string, images []string) error
}
```

Aliases are assigned in order: the first control-plane node is `controlplane`
(the one `kubeadm init` ran on, so every single-control-plane scenario is
unchanged); in an HA cluster the others, sorted by name, are `controlplane02`
and `controlplane03`, mirroring kind's `-control-plane2` and
`-control-plane3`. The workers, sorted by name, are `node01`, `node02`, and so
on. `controlplane01` is rejected so a node has one name.

A scenario asks for an HA cluster with `cluster.controlPlanes: 3` (1 or 3 only:
etcd needs an odd member count, and more nodes do not fit typical Docker
memory). kind then adds an Envoy load balancer container named
`<cluster>-external-load-balancer`. It is not a Kubernetes node (no kubelet,
no containerd), so `Nodes()` skips it and it has no alias; `kind delete`
removes it with the cluster. `Prepare` pulls its image (`LoadBalancerImage`,
kind v0.33.0's `envoyproxy/envoy:v1.36.2`; keep it in step with `mise.toml`) so
the pull does not count against the create timeout. An HA cluster uses about
1.9 GiB across its five containers (measured: 3 control planes at 0.5 to 0.7
GiB each, a worker at 0.15 GiB, the load balancer at 30 MiB) and is created in
about 70 seconds, with a create budget of 9 minutes (5, plus 2 for each
control plane after the first). Exam mode, when it arrives, must refuse to
place a `controlPlanes > 1` scenario on an exam cluster: `ExamCluster` has no
such field.

```go
// internal/kube
type Ref struct {
    Kind, Name, Namespace string // Kind as kubectl accepts it
    Selector, FieldSelector string
    AllNamespaces           bool
}
type Client interface {
    Get(ctx context.Context, r Ref) ([]map[string]any, error) // 0..n objs
    Apply(ctx context.Context, manifest []byte) error
    Exec(ctx context.Context, ns, pod, container string,
        argv []string) (execx.Result, error)
    CanI(ctx context.Context, as, verb, resource, sub, ns string) (bool, error)
    Wait(ctx context.Context, r Ref, forCond string, t time.Duration) error
}
```

`Get` returns 0 objects (not an error) when the resource is NotFound, so
"absent" checks are simple.

```go
// internal/check
type Env struct {
    Kube     kube.Client
    Backend  backend.Backend
    Cluster  string
    Dir      string            // materialized scenario dir
    ScriptEnv []string         // KS_* variables
    Runner   execx.Runner
}
type Result struct {
    Passed  bool
    Message string // why it failed, kept short, no spoilers
}
type Check interface {
    Run(ctx context.Context, env *Env) (Result, error) // err = infra error
}
type Factory func(raw json.RawMessage) (Check, error)
// Register is called from init() in each check file.
func Register(typ string, f Factory)
func Build(spec scenario.CheckSpec) (Check, error)
```

```go
// internal/grade
type CheckOutcome struct {
    ID, Description string
    Weight          int
    Passed          bool
    Message         string
    Err             string
    Duration        time.Duration
}
type Report struct {
    Scenario        string
    Checks          []CheckOutcome
    Earned, Possible int
    Percent         float64
    Passed          bool // Percent >= scenario.PassPercent
    HintsUsed       int
    SolutionViewed  bool
    Elapsed         time.Duration
}
type Grader struct{ /* env, timeouts */ }
func (g *Grader) Grade(ctx context.Context, s *scenario.Scenario) Report
```

Checks run one after another, in order, each with its own timeout and the
optional retry. Infra errors count as failures, are shown separately, and
change the exit code to 2 only if every check errored.

```go
// internal/session: persisted as ~/.kubestronaut/state.json
type State struct {
    Version   int         `json:"version"` // 1
    Mode      string      `json:"mode"`    // "scenario" | "exam"
    ID        string      `json:"id"`      // ULID-like time sortable
    Exam      string      `json:"exam,omitempty"`
    StartedAt time.Time   `json:"startedAt"`
    Deadline  *time.Time  `json:"deadline,omitempty"`
    Clusters  []ClusterRef `json:"clusters"`
    Tasks     []Task      `json:"tasks"`
    Current   int         `json:"current"`
}
type ClusterRef struct {
    Name, Context, AdminKubeconfig, KubernetesVersion string
    Nodes []backend.Node
}
type Task struct {
    Scenario       string
    Cluster        string
    Weight         int
    HintsUsed      int
    SolutionViewed bool
    LastGrade      *grade.Report
}
```

Writes are atomic (temp file plus rename). A `flock` on `~/.kubestronaut/lock`
stops two commands from changing state at the same time.

`internal/lab` is the orchestrator that the CLI, `validate` and exam all use:
`Start`, `Reset`, `Stop`, `Grade`, `Hint`, `RunSolution`. The CLI is a thin
layer over it.

`internal/setup` runs the steps in a scenario's `spec.setup`.
`internal/kubeconfig` builds the user kubeconfig: it renames contexts and
merges with `KUBECONFIG=a:b kubectl config view --flatten --raw`, so there is
no clientcmd dependency.

## 4. Scenario schema

### 4.1 Pack (`scenarios/<pack>/pack.yaml`)

```yaml
apiVersion: kubestronaut/v1alpha1
kind: Pack
metadata:
  name: cka
spec:
  title: Certified Kubernetes Administrator
  passPercent: 66
  domains:            # validated: scenario domain must be listed here
    - {id: cluster-architecture, title: Cluster Architecture, weight: 25}
    - {id: workloads-scheduling, title: Workloads & Scheduling, weight: 15}
    - {id: services-networking, title: Services & Networking, weight: 20}
    - {id: storage, title: Storage, weight: 10}
    - {id: troubleshooting, title: Troubleshooting, weight: 30}
```

### 4.2 Scenario (`scenarios/<pack>/<name>/scenario.yaml`)

The whole file, the prompt, hints and setup manifests are rendered with Go
`text/template` before parsing. Available values:

- `{{node "node01"}}`: the real node name
- `{{.Context}}`
- `{{.Cluster}}`

Scripts are not templated; they get environment variables instead (4.4).

```yaml
apiVersion: kubestronaut/v1alpha1   # required
kind: Scenario                      # required
metadata:
  name: troubleshoot-kubelet        # required, must equal the dir name
  title: Fix a broken kubelet       # required
  domain: troubleshooting           # required, from pack.yaml
  difficulty: medium                # easy | medium | hard
  tags: [kubelet, systemd]          # optional
  experimental: false               # hidden from list/exams unless -a
spec:
  timeLimit: 10m                    # suggested; shown, not enforced
  weight: 7                         # default exam weight, percent
  passPercent: 100                  # default 100
  context: k8s-kubelet              # user kubeconfig context name
  namespaces: [ops]                 # namespaces owned; exam conflicts
  exclusive: true                   # changes node or cluster-wide state
  cluster:
    workers: 1                      # default 0 (single node)
    kubernetesVersion: v1.36.4      # default: engine default
    kindConfigPatches: []           # optional raw kubeadm patches
    networking:
      disableDefaultCNI: false      # true: no CNI, create uses --wait 0s
  images:                           # optional, preloaded into nodes
    - registry.k8s.io/e2e-test-images/agnhost:2.61
  prompt: prompt.md                 # required, relative file
  hints:                            # optional, shown in order
    - Compare paths in /var/lib/kubelet/config.yaml with /etc/kubernetes/pki.
    - Is the service enabled as well as running?
  setup:                            # ordered steps, exactly one key each
    - script: setup/break-kubelet.sh
      timeout: 3m                   # default 3m
    - manifest: setup/pinned.yaml   # file or dir; kubectl apply -f
    - wait:
        ref: {kind: deployment, name: pinned, namespace: ops}
        for: create                 # create | condition=<C> | delete
        timeout: 1m                 # wins over the step timeout; default 2m
  checks:                           # see 4.3
    - id: node01-ready
      description: node01 is Ready
      weight: 3
      type: ready
      ref: {kind: node, name: '{{node "node01"}}'}
  solution:
    script: solution/solve.sh       # required for validate --e2e
    readme: solution/README.md      # shown by `solution`
  validation:
    passBeforeSolution: []          # check ids allowed to pass at baseline
    timeout: 20m                    # e2e budget for this scenario
```

### 4.3 Checks

Fields shared by every check:

```yaml
- id: kebab-id          # required, unique in scenario
  description: text     # required, shown in grade output
  weight: 1             # default 1, integer points
  timeout: 20s          # default 20s
  retry: {attempts: 1, interval: 2s}  # default no retry
  type: <type>
  # type-specific fields
```

`ref` (a resource reference) is
`{kind, name?, namespace?, selector?, fieldSelector?}`.

| type | fields | passes when |
| ---- | ------ | ----------- |
| `resourceExists` | `ref`, `absent: bool` | at least 1 object matches (or 0 if `absent`) |
| `jsonPath` | `ref`, `path`, one matcher, `match: all\|any`, `minItems` | matcher holds for the objects |
| `count` | `ref`, `eq\|gte\|lte` | number of matching objects |
| `ready` | `ref`, `minReady` | Pod Ready, Deployment/STS/DS available, Node Ready, PVC Bound, Job Complete |
| `nodeExec` | `node`, `command: [..]` or `shell: str`, expect | result matches the expectation |
| `podExec` | `ref` (pod), `container`, command/shell, expect | as nodeExec |
| `connectivity` | `from` (pod ref), `to: host:port`, `expect: allowed\|denied`, `probe: agnhost\|nc`, `connectTimeout: 3s` | the probe's outcome matches `expect` |
| `canI` | `as`, `verb`, `resource`, `subresource`, `namespace`, `allowed: bool` | `kubectl auth can-i` result |
| `script` | `path`, `args`, `env` | exit code 0; the last stderr line is the message |

`jsonPath` matchers, exactly one of: `equals`, `notEquals`, `matches`
(RE2), `contains`, `oneOf: [..]`, `gte`, `lte` (numeric),
`empty: true`, `nonEmpty: true`.

The path uses kubectl syntax, with or without braces. Missing keys produce an
empty string, which fails unless `empty: true`.

Exec expectations: `exitCode` (default 0), `stdoutContains`,
`stdoutMatches`.

Connectivity is probed with `kubectl exec <from> -- /agnhost connect
host:port --timeout=3s`, or `nc -z -w3` when `probe: nc`. Source pods must
use agnhost or busybox images; `validate` warns about this.

### 4.4 Script environment (setup, solution, script checks)

Scripts run as `bash <file>` because go:embed does not keep the executable
bit. The working directory is the materialized scenario dir. Environment:

- `KUBECONFIG`: the admin kubeconfig
- `KS_BIN`: path of this binary
- `KS_CLUSTER`
- `KS_CONTEXT`
- `KS_SCENARIO_DIR`
- `KS_NODE_CONTROLPLANE`, `KS_NODE_NODE01`, and so on: real node names
- `KS_LIB`: a helper library you source

`lib.sh` provides:

- `ks_node <alias> <cmd...>`: wraps `"$KS_BIN" ssh <alias> -- cmd`, so
  scripts never call `docker` and stay backend-neutral
- `ks_node_name <alias>`
- `ks_wait_node <alias> Ready|NotReady <secs>`
- `ks_log`

Setup output goes to `sessions/<id>/setup.log` and is shown only with `-v`,
so the user does not see spoilers.

### 4.5 Worked example A: `cka/troubleshoot-kubelet`

`scenario.yaml`:

```yaml
apiVersion: kubestronaut/v1alpha1
kind: Scenario
metadata:
  name: troubleshoot-kubelet
  title: Fix a broken kubelet
  domain: troubleshooting
  difficulty: medium
  tags: [kubelet, systemd, node]
spec:
  timeLimit: 10m
  weight: 7
  context: k8s-kubelet
  namespaces: [ops]
  exclusive: true
  cluster:
    workers: 1
  images:
    - registry.k8s.io/e2e-test-images/agnhost:2.61
  prompt: prompt.md
  hints:
    - Read `journalctl -u kubelet` on node01 for the first error.
    - Compare file paths in /var/lib/kubelet/config.yaml with
      /etc/kubernetes/pki. Then check the unit is enabled, not just started.
  setup:
    - script: setup/break-kubelet.sh
    - manifest: setup/pinned.yaml
  checks:
    - id: kubelet-enabled
      description: kubelet is enabled on node01 (survives reboot)
      weight: 2
      type: nodeExec
      node: node01
      command: [systemctl, is-enabled, kubelet]
    - id: node01-ready
      description: node01 is Ready
      weight: 3
      type: ready
      ref: {kind: node, name: '{{node "node01"}}'}
      retry: {attempts: 10, interval: 3s}
    - id: pinned-ready
      description: deployment ops/pinned is available
      weight: 2
      type: ready
      ref: {kind: deployment, name: pinned, namespace: ops}
      retry: {attempts: 10, interval: 3s}
  solution:
    script: solution/solve.sh
    readme: solution/README.md
  validation:
    passBeforeSolution: []
    timeout: 15m
```

`prompt.md`:

```markdown
Node `node01` is `NotReady` and the workload `ops/pinned` that must run
on it is not running.

Find and fix the cause. Your fix must survive a reboot of the node.
Do not change the `pinned` deployment.

Reach the node with `kubestronaut ssh node01`.
```

`setup/break-kubelet.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

# Two independent faults: a restart alone must not fix the node.
ks_node node01 sed -i \
  's#/etc/kubernetes/pki/ca.crt#/etc/kubernetes/pki/ca.cert#' \
  /var/lib/kubelet/config.yaml
ks_node node01 systemctl disable --now kubelet
# Wait for NotReady so the first grade fails deterministically.
ks_wait_node node01 NotReady 120
```

`setup/pinned.yaml` contains namespace `ops` and deployment `pinned`
(1 replica of agnhost `netexec`). It uses
`nodeSelector: {kubernetes.io/hostname: '{{node "node01"}}'}` and runs with
the NotReady taint left untolerated, so it stays Pending until the node is
fixed.

`solution/solve.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

ks_node node01 sed -i 's#pki/ca.cert#pki/ca.crt#' \
  /var/lib/kubelet/config.yaml
ks_node node01 systemctl enable --now kubelet
kubectl wait node "$(ks_node_name node01)" \
  --for=condition=Ready --timeout=120s
kubectl -n ops rollout status deploy/pinned --timeout=120s
```

### 4.6 Worked example B: `cka/networkpolicy-isolate`

```yaml
apiVersion: kubestronaut/v1alpha1
kind: Scenario
metadata:
  name: networkpolicy-isolate
  title: Restrict access to an API with a NetworkPolicy
  domain: services-networking
  difficulty: medium
  tags: [networkpolicy]
spec:
  timeLimit: 10m
  weight: 6
  context: k8s-netpol
  namespaces: [shop, other]
  exclusive: false
  images:
    - registry.k8s.io/e2e-test-images/agnhost:2.61
  prompt: prompt.md
  hints:
    - A policy that selects a pod denies all ingress not explicitly allowed.
    - Combine podSelector and namespaceSelector carefully; `other` must
      stay blocked.
  setup:
    - manifest: setup/workloads.yaml
    - wait:
        ref: {kind: pod, namespace: shop, selector: app}
        for: condition=Ready
    - wait:
        ref: {kind: pod, name: intruder, namespace: other}
        for: condition=Ready
  checks:
    - id: policy-exists
      description: NetworkPolicy shop/api-allow-web exists
      type: resourceExists
      ref: {kind: networkpolicy, name: api-allow-web, namespace: shop}
    - id: policy-selects-api
      description: the policy selects the api pods
      type: jsonPath
      ref: {kind: networkpolicy, name: api-allow-web, namespace: shop}
      path: '{.spec.podSelector.matchLabels.app}'
      equals: api
    - id: web-allowed
      description: shop/web can reach api on 8080
      weight: 2
      type: connectivity
      from: {kind: pod, name: web, namespace: shop}
      to: api.shop.svc.cluster.local:8080
      expect: allowed
    - id: debug-denied
      description: shop/debug cannot reach api
      weight: 2
      type: connectivity
      from: {kind: pod, name: debug, namespace: shop}
      to: api.shop.svc.cluster.local:8080
      expect: denied
    - id: other-denied
      description: pods in namespace other cannot reach api
      weight: 2
      type: connectivity
      from: {kind: pod, name: intruder, namespace: other}
      to: api.shop.svc.cluster.local:8080
      expect: denied
  solution:
    script: solution/solve.sh
    readme: solution/README.md
  validation:
    passBeforeSolution: [web-allowed]
    timeout: 10m
```

The prompt says: "In namespace `shop`, create a NetworkPolicy named
`api-allow-web` so that only pods labelled `app=web` in `shop` can reach pods
labelled `app=api` on TCP 8080. All other ingress to `api` must be denied. Do
not change other workloads."

`setup/workloads.yaml` creates:

- namespaces `shop` and `other`
- pod and Service `api` (agnhost `netexec --http-port=8080`, `app=api`)
- pods `web` (`app=web`) and `debug` (`app=debug`) in `shop`, running
  `agnhost pause`
- pod `intruder` in `other`

`solution/solve.sh` uses `kubectl apply -f - <<'EOF'` with the policy:
podSelector `app=api`, `policyTypes: [Ingress]`, and ingress from podSelector
`app=web` on port 8080/TCP.

kindnet has enforced NetworkPolicy since kind v0.24, so no Calico is needed;
the e2e validation proves it.

## 5. Cluster lifecycle

- **Naming.** `ks-<pack>-<name>` for one scenario, truncated and hashed to
  at most 40 characters so node hostnames stay under 63. Exam clusters are
  `ks-exam-<cid>`. kubestronaut only ever deletes clusters with the `ks-`
  prefix.
- **kind config.** Generated per scenario and written to
  `sessions/<id>/kind-<cluster>.yaml`:

  ```yaml
  kind: Cluster
  apiVersion: kind.x-k8s.io/v1alpha4
  nodes:
    - role: control-plane
      image: kindest/node:v1.36.4@sha256:099e0493...
    - role: worker
      image: kindest/node:v1.36.4@sha256:099e0493...
  ```

  Created with `kind create cluster --name N --config F --kubeconfig
  sessions/<id>/admin.kubeconfig --wait 3m`. `kindConfigPatches` go into
  `kubeadmConfigPatches`. With `networking.disableDefaultCNI` the config gets
  `networking: {disableDefaultCNI: true}` and kind runs with `--wait 0s`,
  because the nodes cannot become Ready without a CNI.
- **Image table.** `internal/backend/kind/images.go` maps versions to the
  digests published for kind v0.33.0:

  | Version | Digest |
  | ------- | ------ |
  | v1.35.8 | `sha256:07b2536e30b803ed61d1677a79df6115f798ce64c80f9e22f6ed45afd09323c0` |
  | v1.36.4 | `sha256:099e049362a1526b2db71494e1947aae99bd16290d7c895f2b7ea312e3cbfaed` |
  | v1.37.0 | `sha256:a1ed56cfb0e7b93589bdf97c8cd566405a265939e3620fc4f5de89adff580ae5` |

  Copy the full digests from the kind v0.33.0 release notes. The default is
  v1.36.4. An unknown version is an error unless `--node-image` is given.
- **Node image build.** The pinned image has no editor, so `Backend.Prepare`
  builds `kubestronaut/node:<version>-<hash>` on top of it before the first
  create for that version (`internal/backend/kind/nodeimage/Dockerfile`:
  vim, nano, less, dnsutils, procps, net-tools, a YAML-friendly vimrc and the
  `k` alias, plus checksum-verified etcdctl/etcdutl, Helm and the bridge CNI
  plugin, and the ipvsadm packages as .deb files). The etcd release is pinned
  per Kubernetes version in `images.go`. The hash covers the Dockerfile, base
  image, etcd pin and apt pin.
  A version with an `aptReleases` entry (v1.36.4 today) also gets a local,
  flat apt repository of the next minor's kubeadm, kubelet and kubectl in
  `/opt/kubernetes-apt/<minor>/`, so the kubeadm upgrade lab runs the real
  `apt-get install` flow offline. Each deb is pinned by sha256 taken from the
  signed `Packages` index of pkgs.k8s.io (the fingerprint and date of the
  signature check are in `images.go`); the repository itself is `trusted=yes`
  because a key generated in the same build would prove nothing. The
  `kubernetes-cni` dependency of kubelet is met by a small `kind-cni-plugins`
  package that `Provides: kubernetes-cni`, because kind ships CNI v1.9.1
  outside dpkg. A `force-confold` dpkg setting keeps kind's
  `/etc/default/kubelet` when the kubelet package is installed. The source
  file `/opt/kubernetes-apt/kubernetes.list` is not enabled by the image: the
  lab copies it into `/etc/apt/sources.list.d/`, so the other labs never see
  the new packages. The layer adds about 33 MB (arm64) or 38 MB (amd64).
  The build goes to a `-candidate` tag and is promoted only after a smoke
  test confirms the tools exist, so a broken image never sits under the real
  tag. Prepare has its own 15m budget so a slow first build cannot eat into
  the create timeout. Images carry the `io.kubestronaut.node` label, and
  `doctor` lists old ones that can be removed. `reset` keeps a
  `--node-image` override.
- **Kubeconfigs.**
  - The admin kubeconfig belongs to the grader and setup only.
  - `~/.kubestronaut/kubeconfig` is regenerated from all active clusters,
    with contexts renamed to the scenario or exam names and current-context
    set.
  - kubestronaut never reads or writes `~/.kube/config`. `start` prints
    `export KUBECONFIG=...`; `env` and `shell` do the same job.
- **State dir (`$KUBESTRONAUT_HOME`, default `~/.kubestronaut`, mode 0700):**
  - `kubeconfig`
  - `state.json`
  - `lock`
  - `cache/`: image tar cache, reserved
  - `sessions/<id>/`, containing:
    - `admin.kubeconfig`
    - `kind-*.yaml`
    - `scenario/<pack>/<name>/`: materialized files, without `solution/`
    - `setup.log`
    - `grades/*.json`
- **One active session at a time.** `start` with an active session errors
  unless `--force`, which stops the old one first. This fits about 8 GB of
  Docker memory.
- **Images.** For each image in `images`: `docker pull` it if it is not
  present, then `kind load docker-image`. If loading fails, a warning is
  printed and nodes pull the image themselves. Known risk: `kind load` can
  fail under Docker's containerd image store with multi-arch images.
- **Timeouts:**
  - node image prepare: 15m (only a first build per version is slow; e2e
    runs it before the scenario's own budget starts)
  - kind create: 5m (includes `--wait 3m`), plus 2m for each control plane
    after the first
  - image load: 5m
  - each setup script: 3m by default
  - each wait step: 2m
  - whole setup: 15m
  - each check: 20s
  - whole grade: 5m
  - `validate --e2e` per scenario: `validation.timeout`, default 20m
- **Interrupts.** `signal.NotifyContext` cancels the work. If `start` fails
  or is interrupted, the half-built cluster is deleted unless
  `--keep-on-failure`.
- **Cleanup.** `stop` deletes the session's clusters, removes their contexts
  and clears state. `stop --all` and `doctor --fix` also delete orphaned `ks-*`
  clusters.
- **Reset.** Delete and recreate the cluster, then run setup again. This is
  slow (about 1 to 2 minutes) but always gives a clean state. A later idea is
  a fast path for non-exclusive scenarios: delete the owned namespaces and
  re-run setup.
- **Node shell.** `kubestronaut ssh node01` runs
  `docker exec -it <node> bash -l`.

## 6. Initial CKA scenarios

| # | id | domain | nodes | phase |
| - | -- | ------ | ----- | ----- |
| 1 | rbac-dev-access | cluster-architecture | 1 | 1 |
| 2 | networkpolicy-isolate | services-networking | 1 | 1 |
| 3 | troubleshoot-kubelet | troubleshooting | 2 | 1 |
| 4 | etcd-backup-restore | cluster-architecture | 1 | 2 |
| 5 | kubeadm-upgrade | cluster-architecture | 1 | 2 (exp.) |
| 6 | scheduling-taints-affinity | workloads-scheduling | 3 | 2 |
| 7 | deployment-rollback | workloads-scheduling | 1 | 2 |
| 8 | service-fix-selector | services-networking | 1 | 2 |
| 9 | gateway-httproute | services-networking | 1 | 2 |
| 10 | storage-pv-pvc | storage | 1 | 2 |
| 11 | troubleshoot-scheduler | troubleshooting | 1 | 2 |

1. **rbac-dev-access.**
   - Setup: namespace `dev` and a sample deployment.
   - Task: create ServiceAccount `deployer`, a Role (CRUD on deployments,
     get/list on pods), a RoleBinding, and a ClusterRole/Binding so it can
     list nodes.
   - Checks (`canI` as `system:serviceaccount:dev:deployer`): create and
     delete deployments in dev allowed; create deployments in `default`
     denied; get secrets in dev denied; list nodes allowed.
   - `resourceExists` for the RoleBinding. A `script` check confirms no
     ClusterRoleBinding gives the SA `cluster-admin`.
   - `passBeforeSolution`: the two "denied" checks.
2. **networkpolicy-isolate.** See 4.6.
3. **troubleshoot-kubelet.** See 4.5.
4. **etcd-backup-restore** (exclusive).
   - Setup:
     - Copy `etcdctl` and `etcdutl` from the running etcd container:
       `cp /proc/$(pgrep -x etcd)/root/usr/local/bin/etcd* /usr/local/bin/`.
     - Create configmap `vault/golden` and snapshot to
       `/opt/backup/golden.db`.
     - Delete `golden` and create `vault/post-snapshot`.
   - Task: snapshot the current state to `/opt/backup/now.db`, then restore
     `golden.db` into `/var/lib/etcd-golden` and point the etcd static pod at
     it.
   - Checks:
     - `nodeExec etcdutl snapshot status` on now.db
     - a `script` check that now.db's revision is higher than golden.db's
     - `nodeExec` grep that the etcd manifest's hostPath is
       `/var/lib/etcd-golden`
     - `vault/golden` exists and `post-snapshot` is absent
     - controlplane Ready
   - Feasible: the node is privileged, so the `/proc` copy works.
5. **kubeadm-upgrade** (exclusive). Cluster on v1.36.4, two
   nodes.
   - Setup: the node image holds a local apt repository of the v1.37 package
     line (see "Node image build"); setup enables it on both nodes and
     preloads the v1.37.1 control-plane images with `images`, so nothing needs
     internet.
   - Task: upgrade both nodes to v1.37.1 with the real flow: `apt-get
     install kubeadm`, `kubeadm upgrade plan/apply`, drain, kubelet and
     kubectl packages, restart, uncordon, then `kubeadm upgrade node` on the
     worker.
   - Checks: dpkg versions, the `kubeadm-config` ConfigMap, static pod
     manifests and add-on images (only `apply` changes them), kubelet
     versions, schedulable and Ready nodes, and that the `apps/web` pods were
     recreated by a drain.
   - Spike (kind v0.33.0): `apply` needs no preflight ignore and takes about
     160 s, and the whole run about 8 minutes.
     etcd 3.6.8 to 3.7.0 works. The kubelet upgrade restarts the control-plane
     static pods, so the solution waits for the API server to answer steadily
     before the next drain.
   - Passed `validate --e2e` three times in a row on arm64 before it lost its
     `experimental` flag.
5b. **kubeadm-join-node.** Two nodes. Setup drains and resets `node01`,
    deletes its Node object and kind's never-expiring join token. The task is
    a 2 hour bootstrap token, the complete join command written to a file, and
    the join. The file is graded against the live cluster (token secret,
    lifetime, CA hash, endpoint). `kubeadm join` needs no preflight flags in
    kind.
5c. **ha-replace-control-plane** (exclusive). Three control
    planes behind a load balancer, plus a worker (`cluster.controlPlanes: 3`).
    Setup resets and wipes `controlplane03` but leaves its etcd member. The
    task is `etcdctl member remove`, `kubectl delete node`, `kubeadm init
    phase upload-certs`, and a `kubeadm join --control-plane` with the
    certificate key. Joining without removing the member first does not work:
    the new etcd panics against the old member id, and the join hangs. It
    passed `validate --e2e` three times in a row on arm64 before it lost its
    `experimental` flag.
6. **scheduling-taints-affinity.**
   - Setup: node02 tainted `dedicated=gpu:NoSchedule` and labelled
     `accelerator=gpu`.
   - Task: Deployment `ml/trainer` with 2 replicas that runs only on node02
     (a toleration plus required nodeAffinity).
   - Checks: ready (minReady 2); `jsonPath` with `match: all` on pods
     `app=trainer`, `.spec.nodeName == {{node "node02"}}`; the deployment has
     a required affinity on key `accelerator` (stops people hard-coding
     `nodeName`).
7. **deployment-rollback.**
   - Setup: `web` at revision 1 (agnhost) and revision 2 with a bad image
     tag, so the rollout is stuck.
   - Task: roll back, scale to 3, set cpu 100m and memory 64Mi requests.
   - Checks: the image tag is correct; readyReplicas is 3; requests via
     `jsonPath`.
8. **service-fix-selector.**
   - Setup: deployment `catalog` (app=catalog) and Service `catalog` with a
     wrong selector and wrong targetPort.
   - Task: fix the Service and add NodePort Service `catalog-np` on 30080.
   - Checks:
     - EndpointSlice ready count >= 2 (`jsonPath` with selector
       `kubernetes.io/service-name=catalog`)
     - `connectivity` from the client pod
     - `nodeExec` curl to `localhost:30080` on controlplane
     - nodePort == 30080
9. **gateway-httproute.**
   - Setup: Gateway API v1.6.3 standard CRDs (vendored and embedded),
     GatewayClass `ks` with no controller, Gateway `shop/web-gw`, Services
     `api-v1` and `api-v2`.
   - Task: HTTPRoute `api` on `web-gw` with path prefix `/api` and a 90/10
     weight split.
   - Checks: `jsonPath` on parentRefs, matches and backendRefs weights.
   - **Limitation:** there is no data plane, so traffic is not tested and
     status will not be Accepted. A real controller is too heavy for now.
     Ingress is skipped: ingress-nginx was retired in 2026 and Gateway API is
     the current curriculum.
10. **storage-pv-pvc.**
    - Task: create PV `pv-logs` (hostPath `/mnt/logs`, 1Gi, RWO,
      storageClassName `manual`, Retain), PVC `app/logs` bound to it, and pod
      `logger` mounting it at `/var/log/app`.
    - Checks: PV fields; PVC Bound with `volumeName == pv-logs`; pod Ready;
      `podExec` `test -s /var/log/app/out.log`; `nodeExec` showing the file
      exists under `/mnt/logs` (proves hostPath is used).
    - kind's local-path `standard` StorageClass is not involved.
11. **troubleshoot-scheduler** (exclusive).
    - Setup: move `kube-scheduler.yaml` out of `/etc/kubernetes/manifests`
      to `/etc/kubernetes/kube-scheduler.yaml.bak`, with a typo in its
      `--kubeconfig` path; create deployment `app/web`, which stays Pending.
    - Checks: ready, selector `component=kube-scheduler`, in kube-system;
      `app/web` available; `nodeExec` showing the manifest is back in
      `manifests/`.

Backlog: Helm install/upgrade (the user needs helm), Kustomize, CoreDNS
troubleshooting, CRD/operator, PriorityClass, HPA.

## 7. Testing strategy

- **Unit tests** (always run, no Docker):
  - **scenario:** load from `fstest.MapFS`; strict decode rejects unknown
    fields; templating; static validation errors (missing files, duplicate
    ids, unknown domain, unknown node alias, solution missing,
    `passBeforeSolution` naming a check that does not exist).
  - **check:** table tests for every type against `kube.Fake` (objects
    stored in memory, plus programmable exec and can-i responses) and
    `backend/fake`. Covers jsonPath matchers, `all`/`any`, missing keys,
    retry and timeout behaviour.
  - **grade:** weights, partial credit, pass percent, infra errors.
  - **backend/kind:** golden tests for kind config and the commands sent to
    `execx.Fake`; alias assignment.
  - **kubeconfig:** rename and merge using recorded kubectl calls.
  - **session:** atomic writes, locking, schema version.
  - **exam:** cluster assignment validation, the node budget, weighted
    totals, deadline handling with an injected clock.
  - **Embedded packs:** a test runs static `validate` over `scenarios.FS`,
    so a broken scenario fails `go test`.
- **`validate --e2e <id>`**, in `internal/validate`:
  1. Start a fresh cluster.
  2. Run setup.
  3. Grade. Every check must fail except those in `passBeforeSolution`.
  4. Run the solution script.
  5. Grade again; it must reach 100%.
  6. Stop the cluster (always, unless `--keep`).
- **`test/e2e/e2e_test.go`** (`//go:build e2e`) skips unless `KS_E2E=1`. It
  runs the same `validate.E2E` for each id in `KS_E2E_SCENARIOS` (default:
  the phase 1 three), with `t.Cleanup` deleting clusters. Command:
  `go test -tags e2e -timeout 90m ./test/e2e/`.

## 8. Tooling

`mise.toml`:

```toml
[tools]
actionlint = "1.7.12"
go = "1.27.2"
golangci-lint = "2.14.0"
kind = "0.33.0"
kubectl = "1.37.1"
markdownlint-cli2 = "0.23.3"   # npm backend, needs node
node = "26.11.1"
pre-commit = "4.6.2"
shellcheck = "0.11.0"
shfmt = "3.14.1"

[tasks.build]
run = "go build -o bin/kubestronaut ./cmd/kubestronaut"
[tasks.test]
run = "go test -race -timeout 5m ./..."
[tasks."lint:go"]
run = "golangci-lint run --timeout 5m"
[tasks."lint:md"]
run = "markdownlint-cli2 '**/*.md'"
[tasks."lint:sh"]
run = """
files=$(git ls-files '*.sh')
shellcheck $files
shfmt -d -i 2 -ci $files
"""
[tasks."lint:actions"]
run = "actionlint"
[tasks.lint]
depends = ["lint:go", "lint:md", "lint:sh", "lint:actions"]
[tasks.validate]
run = "go run ./cmd/kubestronaut validate --all"
[tasks.e2e]
run = "KS_E2E=1 go test -tags e2e -timeout 90m ./test/e2e/"
[tasks.ci]
depends = ["lint", "test", "validate"]
```

- **`.golangci.yml`** (v2 format, `version: "2"`):
  - Linters: errcheck, govet, staticcheck, ineffassign, unused, revive,
    gosec, errorlint, gocritic, misspell, nolintlint, unparam.
  - Formatters: gofmt, goimports.
  - gosec G204 is excluded only in `internal/execx`, because launching
    subprocesses is that package's whole job.
- **`.markdownlint-cli2.yaml`:** MD013 `line_length: 80`, with `tables:
  false` (wide tables are clearer); ignore `node_modules`.
- **`.pre-commit-config.yaml`:**
  - `pre-commit-hooks` pinned to its latest v6.x tag: trailing-whitespace,
    end-of-file-fixer, check-yaml (with `--allow-multiple-documents`),
    check-added-large-files.
  - Local hooks that call `mise run lint:go`, `lint:md`, `lint:sh`, `test`
    (so hook versions match mise).
- **`.github/workflows/ci.yml`:**
  - Triggers: push to main and pull requests. `permissions: contents:
    read`.
  - Job `lint` (`timeout-minutes: 15`, ubuntu-24.04): checkout, then
    `jdx/mise-action` v5.1.1, then `mise run lint`.
  - Job `test` (15 min): `mise run test` and `mise run validate`.
  - Pin actions by full commit SHA with a version comment
    (`actions/checkout` v7.0.1, `jdx/mise-action` v5.1.1).
- **`.github/workflows/e2e.yml`:**
  - Triggers: `workflow_dispatch` (input `scenarios`), a weekly schedule,
    and pull requests carrying the `e2e` label.
  - A matrix job builds the id list from
    `kubestronaut list --exam cka --json`.
  - Per-scenario job: `timeout-minutes: 55`, which is more than the
    go test timeout (`KS_E2E_TIMEOUT: 45m`), itself more than the scenario's
    `validation.timeout` plus about 10m of tool install and kind image
    pulls. The longest is 25m today; a unit test caps it at 30m.
  - Step: `mise run e2e` with `KS_E2E_SCENARIOS=${{ matrix.id }}`. kind
    comes from mise; Docker is preinstalled on the runner.
- **Timeout rule:** each job timeout must be greater than the tool timeout
  plus setup. Unit job 15m > go test 5m; lint 15m > golangci 5m; e2e job 55m >
  go test 45m > validation 30m.

## 9. Phased implementation plan

### Phase 1: engine, kind backend, CLI, 3 scenarios, tests

Do the steps in order. Each ends with `mise run ci` green.

1. **Bootstrap.**
   - `go mod init github.com/JoshSLawrence/kubestronaut` and add the three
     deps.
   - Add mise.toml, linters, pre-commit, .gitignore (`bin/`), a LICENSE
     placeholder, a README stub, and `cmd/kubestronaut/main.go` with a cobra
     root and `version`.
2. **`execx`:** the real runner plus a fake that records `Cmd` and returns
   queued results matched by argv prefix.
3. **`scenario`:** types for Pack, Scenario, CheckSpec, SetupStep, Exam
   (exam types only; the logic comes in phase 2).
   - YAML loading over `fs.FS`, layered: embedded first, then the dir
     override.
   - Templating with `node`, `.Context`, `.Cluster`.
   - Static validation exactly as section 7 lists it.
   - Unit tests.
4. **`kube`:** a kubectl-backed `Client` plus `Fake`.
   - `Get` uses `kubectl get <kind> [name] -n ns -l sel -o json`; a list
     result is flattened into its items.
5. **`check`:** registry plus all 9 types (4.3), with unit tests per type.
6. **`grade`:** Report, scoring, and text and JSON output in `ui`.
7. **`backend`:** interface, `fake`, and `kind` (create, delete, list,
   nodes and aliases, exec and shell through `docker exec`, LoadImages, the
   image table). Golden tests.
8. **`kubeconfig` and `session`:** state dir, lock, atomic state, user
   kubeconfig generation.
9. **`setup`:** step runner, the embedded `lib.sh` written to the session
   dir, the script env, and logs.
10. **`lab` and `cli`:**
    - list, show, start, status, prompt, env, shell, ssh, hint, grade,
      solution, reset, stop, validate (static and `--e2e`), doctor.
    - Exit codes as in 1.1.
11. **Scenarios:** pack.yaml; rbac-dev-access, networkpolicy-isolate,
    troubleshoot-kubelet, each with prompt, setup, checks and solution.
12. **`test/e2e`, `e2e.yml`, and `docs/authoring.md`.**

Phase 1 is done when, on a dev machine,
`kubestronaut validate --e2e --all` passes for all 3 scenarios, and the
unit-test coverage of `check`, `grade` and `scenario` is at least 80%.

### Phase 2: remaining scenarios and exam mode

1. **Spike** kubeadm-upgrade and etcd-backup-restore by hand on kind and
   record the findings in their solution READMEs. If kubeadm-upgrade cannot
   pass `validate --e2e` reliably, keep it `experimental: true`.
2. **Scenarios 4 and 6 to 11,** one at a time, each passing `validate --e2e`
   before the next starts. Vendor Gateway API v1.6.3
   `standard-install.yaml` into `gateway-httproute/setup/`.
3. **`exam`:**
   - Exam YAML in `<pack>/exams/*.yaml`:

     ```yaml
     apiVersion: kubestronaut/v1alpha1
     kind: Exam
     metadata: {name: mock-1}
     spec:
       title: CKA mock exam 1
       duration: 120m
       passPercent: 66
       allowGrade: false
       maxNodes: 5
       clusters:
         - {name: c1, context: k8s-c1, workers: 1}
         - {name: c2, context: k8s-c2, workers: 1}
       tasks:
         - {scenario: cka/rbac-dev-access, cluster: c1, weight: 4}
         - {scenario: cka/networkpolicy-isolate, cluster: c1, weight: 6}
         - {scenario: cka/troubleshoot-kubelet, cluster: c2, weight: 7}
     ```

   - Validation rules:
     - An exclusive task must be alone on its cluster.
     - Non-exclusive tasks must have disjoint `namespaces`.
     - Each cluster must have at least as many workers as its tasks need.
     - The total node count must be at most `maxNodes`.
     - A scenario's `spec.context` is replaced by the cluster's context.
   - `exam start` creates clusters one after another, runs every task's
     setup, and stores the deadline. `exam task`, `exam status` and
     `exam submit` behave as in 1.1.
   - Scoring: each task scores weight times its percent (partial credit),
     normalized to 100.
   - Without `--practice`, `hint` and `solution` are refused and `grade` is
     refused while `allowGrade` is false.
   - After the deadline, `status` shows TIME UP and submit records the
     overtime.
4. **`stop`** for exams; `validate` for exam files.
5. **Write `mock-1`** with 6 or more tasks across all domains, within
   5 nodes.

Phase 2 is done when all non-experimental scenarios pass `validate --e2e`
and `exam start cka/mock-1` then `exam submit` works end to end.
