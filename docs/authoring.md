# Authoring scenarios and packs

A scenario is a directory of plain files. The engine needs no code changes to
run a new one, and a new certification pack (CKAD, CKS, ...) is just a new
directory of scenarios.

## Anatomy of a scenario

```text
scenarios/cka/rbac-dev-access/
├── checks/            optional scripts used by `script` checks
│   └── no-cluster-admin.sh
├── prompt.md
├── scenario.yaml
├── setup/
│   └── workloads.yaml
└── solution/
    ├── README.md
    └── solve.sh
```

The directory name must equal `metadata.name`, and the id the learner types is
`<pack>/<name>`.

## Add a scenario

1. Create `scenarios/<pack>/<name>/` and copy the closest existing scenario as
   a starting point.
2. Write `prompt.md`: what is broken or wanted, in the words of an exam task.
   Do not give the answer away. It is shown verbatim, so start with prose
   rather than a heading.
3. Write `scenario.yaml` (reference below) with `setup` steps that build the
   starting state and `checks` that grade the end state.
4. Write `solution/solve.sh` so that a clean run of it makes every check pass,
   and `solution/README.md` explaining the fix.
5. Prove it:

   ```bash
   go run ./cmd/kubestronaut validate cka/<name>          # static
   go run ./cmd/kubestronaut validate --e2e cka/<name>    # real cluster
   ```

   `--e2e` starts a cluster, runs setup, requires every check to fail (except
   those in `validation.passBeforeSolution`), runs the solution, and requires
   a 100% grade. Use `--keep` to leave the cluster up when something fails.
6. Run `mise run ci`. A test validates every embedded scenario, so a broken
   one fails `go test`.

To try a scenario without rebuilding, point `--scenarios-dir` at a directory
laid out like `scenarios/`. A scenario directory there replaces a built-in one
of the same id completely.

## scenario.yaml reference

```yaml
apiVersion: kubestronaut/v1alpha1
kind: Scenario
metadata:
  name: troubleshoot-kubelet   # equals the directory name
  title: Fix a broken kubelet
  domain: troubleshooting      # must be listed in the pack
  difficulty: medium           # easy | medium | hard
  tags: [kubelet, systemd]
  experimental: false          # hidden from `list` unless -a
spec:
  timeLimit: 10m               # suggested, shown but not enforced
  weight: 7                    # default exam weight in percent
  passPercent: 100             # default 100
  context: k8s-kubelet         # kubeconfig context name
  namespaces: [ops]            # namespaces the scenario owns
  exclusive: true              # changes node or cluster-wide state
  cluster:
    workers: 1                 # default 0, a single node
    kubernetesVersion: v1.36.4 # default: the engine default
  images:                      # preloaded into the nodes
    - registry.k8s.io/e2e-test-images/agnhost:2.61
  prompt: prompt.md
  hints: [first hint, second hint]
  setup: []                    # see below
  checks: []                   # see below
  solution:
    script: solution/solve.sh
    readme: solution/README.md
  validation:
    passBeforeSolution: []     # check ids allowed to pass at baseline
    timeout: 15m               # end-to-end budget, default 20m
```

Unknown fields are rejected, so typos fail validation instead of being ignored.

### Templating

`scenario.yaml`, the prompt and every `.yaml`, `.yml` and `.json` file are
rendered with Go `text/template` before use:

- `{{node "node01"}}` expands to the real node name (for example
  `ks-cka-demo-worker`), not the alias; aliases are `controlplane`, `node01`...
- `{{.Context}}` and `{{.Cluster}}`

Scripts are not templated; they get environment variables instead. In YAML,
quote template calls: `name: '{{node "node01"}}'`.

#### Verbatim files

Everything under the scenario's `files/` directory is copied as it is, even
`.yaml` and `.json` files. Use it for content that has template syntax of its
own, such as a Helm chart's `templates/*.yaml`, or that is vendored and must
keep its checksum. Setup scripts push it to a node with `ks_copy_to`, for
example `ks_copy_to controlplane files/charts /opt/charts`. The directory is
deleted from the learner's scenario directory once setup has run. Go's
`embed` skips files whose names start with `.` or `_`, so a chart cannot use
`_helpers.tpl`.

### Setup steps

Steps run in order and each has exactly one key.

```yaml
setup:
  - script: setup/break-kubelet.sh   # run with bash, timeout default 3m
    timeout: 3m
  - manifest: setup/pinned.yaml      # file or directory, kubectl apply -f
  - wait:
      ref: {kind: deployment, name: web, namespace: ops}
      for: condition=Available       # or create, or delete
      timeout: 2m                    # wins over the step timeout; default 2m
```

A `wait` first waits for the object to exist, so it is safe right after an
apply. Setup output goes to `setup.log` in the session directory (mode 0600, it
describes the fault) and is shown with `-v`. Setup scripts and manifests are
deleted from the learner's scenario directory once setup has run.

### Script environment

Scripts for setup, solution and `script` checks run as `bash <file>` from the
materialized scenario directory, with:

| Variable | Meaning |
| -------- | ------- |
| `KUBECONFIG` | the admin kubeconfig of the scenario cluster |
| `KS_BIN` | path of the kubestronaut binary |
| `KS_CLUSTER`, `KS_CONTEXT` | cluster name, context name |
| `KS_SCENARIO_DIR` | the materialized scenario directory |
| `KS_NODE_CONTROLPLANE`, `KS_NODE_NODE01`, ... | real node names |
| `KS_LIB` | a helper library to `source` |

The library provides `ks_node <alias> <cmd...>` (run on a node through
`kubestronaut ssh`, so scripts never call docker), `ks_node_name <alias>`,
`ks_wait_node <alias> Ready|NotReady <secs>`, `ks_log`, and the helpers below.

Scripts start with `#!/usr/bin/env bash` and `set -euo pipefail`, are
`chmod +x`, and must pass shellcheck and `shfmt -i 2 -ci`. They run on the
learner's machine, which may be macOS with bash 3.2 and BSD tools, so avoid
bash 4 features (`mapfile`, `readarray`, `declare -A`, `${var,,}`, `|&`) and
GNU-only `sed` or `date` syntax. A test rejects the bash 4 constructs.

#### Files on nodes

`ks_copy_to <alias> <local-dir> <node-dir>` copies the contents of a local
directory (for example one the script writes to a temporary directory) onto a
node and creates the destination. It streams a tar archive through `ks_node`,
so the node needs nothing but `mkdir` and `tar`. The archive carries no
macOS metadata files, and the files on the node belong to the node's user
with default permissions, not the host's uid and mode.

#### Facts only setup knows

Setup often creates a value the checks must compare against later: a random
token, an object uid, a file hash. Store it with `ks_answer_put <key> <value>`
in setup and verify it in a `script` check with `ks_answer_check <key>
<value>`, which exits 0 on a match. It prints nothing on a mismatch and logs
a message when no answer was recorded. In setup:

```bash
ks_answer_put token "$token"
```

In the check:

```bash
ks_answer_check token "$(ks_node controlplane cat /root/token.txt)"
```

Only the sha256 of the value is written, to `$KS_SCENARIO_DIR/.answers/<key>`
with mode 0600 in a directory with mode 0700. The learner can read the hash
but cannot reverse a random value, and the answers survive the removal of
setup files. Keys are ASCII letters, digits, `.`, `_` and `-`; anything else
fails so a key cannot escape the directory.

### Tools on the nodes

Nodes run kubestronaut's node image (see the README), so prompts and
solutions can rely on `vim`, `nano`, `less`, `etcdctl`, `nslookup`/`dig`,
`ps` and `netstat` on every node, plus the kind image's `curl`, `jq` and
`crictl`. If a scenario needs another tool, add it to
`internal/backend/kind/nodeimage/Dockerfile` (and to `nodeTools` if its
absence should fail the build) rather than installing it during setup: setup
must not need internet, and every scenario should see the same node.

## Checks

Every check has `id` (kebab-case, unique), `description`, and optional
`weight` (default 1), `timeout` (default 20s, per attempt) and
`retry: {attempts, interval}`. Descriptions and failure messages are shown to
the learner before they have solved anything, so they must not name the fault
or the answer.

A `ref` is `{kind, name?, namespace?, selector?, fieldSelector?}`. A kind the
cluster does not know, such as a custom resource before its operator is
installed, reads as zero objects instead of an error, so checks on it fail
cleanly at baseline. Because a typo reads the same way, every kind used by a
built-in scenario must be listed in `knownKinds` in
`internal/validate/embedded_test.go`.

| Type | Fields | Passes when |
| ---- | ------ | ----------- |
| `resourceExists` | `ref`, `absent` | an object matches (or none if `absent`) |
| `jsonPath` | `ref`, `path`, one matcher, `match: all\|any`, `minItems` | the matcher holds |
| `count` | `ref`, `eq` or `gte`/`lte` | the number of matches fits |
| `ready` | `ref`, `minReady` | Pod/Node Ready, Deployment/StatefulSet/DaemonSet rolled out, PVC Bound, Job Complete |
| `nodeExec` | `node`, `command` or `shell`, expectations | the result matches |
| `podExec` | `ref`, `container`, `command` or `shell`, expectations | the result matches |
| `connectivity` | `from`, `to: host:port`, `expect: allowed\|denied`, `probe`, `connectTimeout` | the probe agrees |
| `canI` | `as`, `verb`, `resource`, `subresource`, `namespace`, `allowed` | `kubectl auth can-i` agrees |
| `script` | `path`, `args`, `env` | the script exits 0; its last stderr line is the message |

`jsonPath` matchers, exactly one: `equals`, `notEquals`, `matches` (RE2),
`contains`, `oneOf`, `gte`/`lte` (numeric, combinable), `empty: true`,
`nonEmpty: true`. The path uses kubectl syntax with or without braces, and a
missing key is an empty string.

Command expectations are `exitCode` (default 0), `stdoutContains` and
`stdoutMatches`. `shell: "..."` runs under `bash -c`.

`connectivity` execs `/agnhost connect` in the source pod (`probe: nc` uses
`nc -z`), so source pods must run agnhost or busybox images. A `denied` check
passes only on a timeout or refusal, never on a DNS failure. A source pod
that exists but has no running container (Pending, not scheduled, or every
container waiting or terminated) fails the check with "not running" rather
than reporting an error, so scenarios can probe from a pod that only starts
once the learner has fixed something. A crash-looping pod has the phase
Running, so it counts as not running only while its container is down. A
source pod that does not exist at all is still an error: that is a scenario
bug.

YAML pitfalls: `true`, `yes` and `no` become booleans, so write
`command: ['true']` and `stdoutContains: 'yes'`.

### Designing good checks

- Check outcomes, not exact commands: several solutions should pass.
- Give "denied" and "must not change" checks too, or the cheapest wrong fix
  passes. Those checks usually pass at baseline; list them in
  `validation.passBeforeSolution`.
- Add `retry` wherever the cluster converges asynchronously (node readiness,
  rollouts, network policy enforcement).

## Add a pack

A pack groups scenarios for one certification.

1. Create `scenarios/<pack>/pack.yaml`:

   ```yaml
   apiVersion: kubestronaut/v1alpha1
   kind: Pack
   metadata:
     name: ckad               # equals the directory name
   spec:
     title: Certified Kubernetes Application Developer
     passPercent: 66
     domains:                 # scenario domains must be listed here
       - {id: application-design, title: Application Design, weight: 20}
   ```

2. Add scenarios under `scenarios/<pack>/<name>/` as above.
3. Nothing else: `scenarios/embed.go` embeds every top-level directory.
4. Run `mise run ci`, then `validate --e2e` for each new scenario.

Packs outside the repository work too: put the same layout under a directory
and pass `--scenarios-dir`.
