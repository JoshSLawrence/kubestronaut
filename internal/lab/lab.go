// Package lab orchestrates a practice session: it is the layer the CLI,
// validate and (later) exam mode all share.
package lab

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/JoshSLawrence/kubestronaut/internal/backend"
	"github.com/JoshSLawrence/kubestronaut/internal/check"
	"github.com/JoshSLawrence/kubestronaut/internal/execx"
	"github.com/JoshSLawrence/kubestronaut/internal/grade"
	"github.com/JoshSLawrence/kubestronaut/internal/kube"
	"github.com/JoshSLawrence/kubestronaut/internal/kubeconfig"
	"github.com/JoshSLawrence/kubestronaut/internal/scenario"
	"github.com/JoshSLawrence/kubestronaut/internal/session"
	"github.com/JoshSLawrence/kubestronaut/internal/setup"
	"github.com/JoshSLawrence/kubestronaut/internal/ui"
)

// Timeouts from the design (section 5).
const (
	// prepareTimeout covers a first-time node image build: an apt install
	// over a slow connection, well past the seconds it normally takes.
	prepareTimeout = 15 * time.Minute
	// createTimeout is for one control plane; createTimeoutFor adds time for
	// each further one, which kind joins one after the other.
	createTimeout = 5 * time.Minute
	loadTimeout   = 5 * time.Minute
	cleanupBudget = 2 * time.Minute
)

// Lab ties together the scenario catalog, a backend and the state store.
type Lab struct {
	Catalog *scenario.Catalog
	Backend backend.Backend
	Store   *session.Store
	Runner  execx.Runner
	// Binary is the path scripts get as KS_BIN.
	Binary  string
	Out     io.Writer
	Verbose bool
	Now     func() time.Time
	// KubeFor builds a client for an admin kubeconfig.
	KubeFor func(admin string) kube.Client
}

// New builds a Lab with real kubectl clients.
func New(cat *scenario.Catalog, be backend.Backend, store *session.Store, r execx.Runner, binary string, out io.Writer) *Lab {
	l := &Lab{Catalog: cat, Backend: be, Store: store, Runner: r, Binary: binary, Out: out, Now: time.Now}
	l.KubeFor = func(admin string) kube.Client {
		return &kube.Kubectl{Runner: r, Kubeconfig: admin, CacheDir: filepath.Join(store.Root, "cache")}
	}
	return l
}

// StartOptions tune Start.
type StartOptions struct {
	K8sVersion    string
	NodeImage     string
	Force         bool
	KeepOnFailure bool
}

// scenarioDir is where a session's scenario files are materialized.
func (l *Lab) scenarioDir(id string, s *scenario.Scenario) string {
	return filepath.Join(l.Store.SessionDir(id), "scenario", s.Pack, s.DirName)
}

// step prints a progress line and returns a function that reports elapsed time.
func (l *Lab) step(format string, a ...any) func() {
	ui.Step(l.Out, format, a...)
	start := l.Now()
	return func() {
		if d := l.Now().Sub(start); d >= 2*time.Second {
			_, _ = fmt.Fprintf(l.Out, "    done in %s\n", ui.Duration(d))
		}
	}
}

// detached returns a context that survives cancellation of ctx, for cleanup
// that must still run after an interrupt.
func detached(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), cleanupBudget)
}

// Start creates a cluster for the scenario, runs its setup and prints the
// prompt.
func (l *Lab) Start(ctx context.Context, ref string, o StartOptions) error {
	unlock, err := l.Store.Lock()
	if err != nil {
		return err
	}
	defer unlock()
	s, err := l.Catalog.Find(ref)
	if err != nil {
		return err
	}
	// A broken scenario should fail in a second, not after a cluster boots.
	if errs := append(s.Validate(l.Catalog.Packs[s.Pack]), check.Validate(s)...); len(errs) > 0 {
		return fmt.Errorf("scenario %s is invalid, not starting a cluster: %w", s.ID, errors.Join(errs...))
	}
	switch st, err := l.Store.Load(); {
	case err == nil:
		if !o.Force {
			return fmt.Errorf("a session for %s is already active: run `kubestronaut stop` first, or pass --force to replace it", st.Tasks[min(max(st.Current, 0), len(st.Tasks)-1)].Scenario)
		}
		if err := l.stopLocked(ctx, st); err != nil {
			return err
		}
	case !errors.Is(err, session.ErrNoSession):
		return err
	}
	return l.startLocked(ctx, s, o)
}

func (l *Lab) startLocked(ctx context.Context, s *scenario.Scenario, o StartOptions) (err error) {
	now := l.Now()
	id := session.NewID(now)
	dir := l.Store.SessionDir(id)
	name := scenario.ClusterName(s.Pack, s.DirName)
	// Refuse before anything is armed for cleanup: the failure path deletes
	// the cluster, and it must only ever delete one this start created.
	existing, err := l.Backend.List(ctx)
	if err != nil {
		return fmt.Errorf("list clusters: %w", err)
	}
	if slices.Contains(existing, name) {
		return fmt.Errorf("a cluster named %s already exists (another --home, or a leftover): run `kubestronaut stop --all` to remove it, then retry", name)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	version := o.K8sVersion
	if version == "" {
		version = s.Spec.Cluster.KubernetesVersion
	}
	if version == "" {
		version = backend.DefaultKubernetesVersion
	}
	admin := filepath.Join(dir, "admin.kubeconfig")
	st := &session.State{
		Mode: session.ModeScenario, ID: id, StartedAt: now,
		Clusters: []session.ClusterRef{{Name: name, Context: s.Spec.Context, AdminKubeconfig: admin, KubernetesVersion: version, NodeImage: o.NodeImage}},
		Tasks:    []session.Task{{Scenario: s.ID, Cluster: name, Weight: s.Spec.Weight}},
	}
	// Saved before the cluster exists so that `stop` can clean up even if
	// this process is killed half way.
	if err := l.Store.Save(st); err != nil {
		return err
	}
	defer func() {
		if err == nil {
			return
		}
		if o.KeepOnFailure {
			ui.Warn(l.Out, "start failed; the cluster %s was kept for debugging. Run `kubestronaut stop` to remove it.", name)
			return
		}
		cctx, cancel := detached(ctx)
		defer cancel()
		if derr := l.Backend.Delete(cctx, name); derr != nil {
			ui.Warn(l.Out, "could not delete cluster %s: %v. Run `kubestronaut stop --all` to retry.", name, derr)
		}
		_ = l.Store.Clear()
		_ = l.regenKubeconfig(cctx, nil)
		_ = os.RemoveAll(dir)
	}()

	workers := s.Spec.Cluster.Workers
	spec := backend.ClusterSpec{
		Name: name, KubernetesVersion: version, NodeImage: o.NodeImage, ControlPlanes: s.Spec.Cluster.CPs(),
		Workers: workers, Kubeconfig: admin, KubeadmPatches: s.Spec.Cluster.KindConfigPatches,
		DisableDefaultCNI: s.Spec.Cluster.Networking.DisableDefaultCNI,
	}
	// Prepare gets its own budget so a first-time image build cannot eat
	// into the cluster create timeout.
	pctx, cancel := context.WithTimeout(ctx, prepareTimeout)
	var built func()
	err = l.Backend.Prepare(pctx, spec, func(msg string) { built = l.step("%s", msg) })
	cancel()
	if err != nil {
		return fmt.Errorf("prepare cluster: %w", err)
	}
	if built != nil {
		built()
	}
	done := l.step("Creating cluster %s (%s, %d node(s))", name, version, s.Spec.Cluster.CPs()+workers)
	cctx, cancel := context.WithTimeout(ctx, createTimeoutFor(spec.ControlPlanes))
	err = l.Backend.Create(cctx, spec)
	cancel()
	if err != nil {
		return fmt.Errorf("create cluster: %w", err)
	}
	done()

	nodes, err := l.Backend.Nodes(ctx, name)
	if err != nil {
		return err
	}
	st.Clusters[0].Nodes = nodes
	if err := l.Store.Save(st); err != nil {
		return err
	}
	if err := l.regenKubeconfig(ctx, st); err != nil {
		return err
	}

	if len(s.Spec.Images) > 0 {
		done = l.step("Loading %d image(s) into the nodes", len(s.Spec.Images))
		lctx, cancel := context.WithTimeout(ctx, loadTimeout)
		lerr := l.Backend.LoadImages(lctx, name, s.Spec.Images)
		cancel()
		if lerr != nil {
			// Not fatal: the nodes pull whatever is missing themselves.
			ui.Warn(l.Out, "image preload failed, nodes will pull images on demand: %s", firstLine(lerr))
		}
		done()
	}

	runner, logFile, err := l.setupRunner(st, s)
	if err != nil {
		return err
	}
	defer func() { _ = logFile.Close() }()
	done = l.step("Running setup (%d steps)", len(s.Spec.Setup))
	if err := runner.Run(ctx, s); err != nil {
		return fmt.Errorf("%w (details: %s, or rerun with -v)", err, logFile.Name())
	}
	done()
	l.removeSetupFiles(st, s)

	st.StartedAt = l.Now()
	if err := l.Store.Save(st); err != nil {
		return err
	}
	ui.Step(l.Out, "Ready.")
	_, _ = fmt.Fprintln(l.Out)
	_, _ = fmt.Fprint(l.Out, l.promptText(s))
	return nil
}

// removeSetupFiles deletes the setup scripts and manifests, and the verbatim
// files/ directory, from the learner's scenario directory once setup ran,
// because they describe the fault or are only material for setup.
func (l *Lab) removeSetupFiles(st *session.State, s *scenario.Scenario) {
	dir := l.scenarioDir(st.ID, s)
	protected := s.CheckScriptPaths()
	paths := []string{scenario.VerbatimDir}
	for _, step := range s.Spec.Setup {
		paths = append(paths, step.Script, step.Manifest)
	}
	for _, p := range paths {
		// Defence in depth: validation rejects these, but a wrong path
		// here would delete the learner's whole directory.
		if p == "" || !filepath.IsLocal(p) || filepath.Clean(p) == "." {
			continue
		}
		keep := false
		for _, c := range protected {
			if scenario.Overlaps(p, c) {
				keep = true
			}
		}
		if !keep {
			_ = os.RemoveAll(filepath.Join(dir, p))
		}
	}
}

// setupRunner materializes the scenario and prepares the step runner. The
// returned file is the setup log; with Verbose it is also echoed.
func (l *Lab) setupRunner(st *session.State, s *scenario.Scenario) (*setup.Runner, *os.File, error) {
	c := st.Clusters[0]
	dir := l.scenarioDir(st.ID, s)
	data := &scenario.TemplateData{Cluster: c.Name, Context: s.Spec.Context}
	if err := s.Materialize(dir, data); err != nil {
		return nil, nil, err
	}
	lib, err := setup.WriteLib(l.Store.SessionDir(st.ID))
	if err != nil {
		return nil, nil, err
	}
	logFile, err := os.OpenFile(filepath.Join(l.Store.SessionDir(st.ID), "setup.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, nil, err
	}
	var w io.Writer = logFile
	if l.Verbose {
		w = io.MultiWriter(logFile, l.Out)
	}
	return &setup.Runner{
		Kube: l.KubeFor(c.AdminKubeconfig), Exec: l.Runner, Log: w, Dir: dir,
		Env: l.scriptEnv(c, dir, lib),
	}, logFile, nil
}

func (l *Lab) scriptEnv(c session.ClusterRef, scenarioDir, lib string) []string {
	return setup.ScriptEnv(setup.EnvParams{
		CacheDir:        filepath.Join(l.Store.Root, "cache"),
		AdminKubeconfig: c.AdminKubeconfig, Binary: l.Binary, Home: l.Store.Root,
		Cluster: c.Name, Context: c.Context, ScenarioDir: scenarioDir, Lib: lib, Nodes: c.Nodes,
	})
}

// regenKubeconfig rewrites the user kubeconfig from the clusters of st.
func (l *Lab) regenKubeconfig(ctx context.Context, st *session.State) error {
	var entries []kubeconfig.Entry
	current := ""
	if st != nil {
		for _, c := range st.Clusters {
			entries = append(entries, kubeconfig.Entry{Admin: c.AdminKubeconfig, Cluster: c.Name, Context: c.Context})
		}
		if len(st.Clusters) > 0 {
			current = st.Clusters[0].Context
		}
	}
	return kubeconfig.Generate(ctx, l.Runner, entries, current, l.Store.KubeconfigPath())
}

func (l *Lab) promptText(s *scenario.Scenario) string {
	var b strings.Builder
	fmt.Fprintf(&b, "  Context: %s", s.Spec.Context)
	if s.Spec.TimeLimit > 0 {
		fmt.Fprintf(&b, "      Suggested time: %s", ui.Duration(s.Spec.TimeLimit.D()))
	}
	if s.Spec.Weight > 0 {
		fmt.Fprintf(&b, "      Weight: %d%%", s.Spec.Weight)
	}
	b.WriteString("\n\n")
	for _, line := range strings.Split(strings.TrimRight(s.Prompt, "\n"), "\n") {
		b.WriteString(strings.TrimRight("  "+line, " ") + "\n")
	}
	fmt.Fprintf(&b, "\n  export KUBECONFIG=%s\n", l.Store.KubeconfigPath())
	return b.String()
}

// current loads the active session and its scenario.
func (l *Lab) current() (*session.State, *session.Task, *scenario.Scenario, error) {
	st, err := l.Store.Load()
	if err != nil {
		return nil, nil, nil, err
	}
	if st.Current < 0 || st.Current >= len(st.Tasks) {
		return nil, nil, nil, fmt.Errorf("state file is inconsistent (no current task): run `kubestronaut stop --all`")
	}
	t := &st.Tasks[st.Current]
	s, err := l.Catalog.Find(t.Scenario)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("the active scenario is no longer available: %w", err)
	}
	return st, t, s, nil
}

// Prompt returns the prompt text of the active scenario.
func (l *Lab) Prompt() (string, error) {
	_, _, s, err := l.current()
	if err != nil {
		return "", err
	}
	return l.promptText(s), nil
}

// Status describes the active session.
type Status struct {
	Scenario   string
	Title      string
	Cluster    string
	Context    string
	Version    string
	Nodes      []backend.Node
	Elapsed    time.Duration
	TimeLimit  time.Duration
	HintsUsed  int
	Solution   bool
	LastGrade  *grade.Report
	Kubeconfig string
}

// Status reports the active session.
func (l *Lab) Status() (*Status, error) {
	st, t, s, err := l.current()
	if err != nil {
		return nil, err
	}
	c, _ := st.Cluster(t.Cluster)
	out := &Status{
		Scenario: s.ID, Title: s.Metadata.Title, Cluster: t.Cluster, Elapsed: l.Now().Sub(st.StartedAt),
		TimeLimit: s.Spec.TimeLimit.D(), HintsUsed: t.HintsUsed, Solution: t.SolutionViewed,
		LastGrade: t.LastGrade, Kubeconfig: l.Store.KubeconfigPath(),
	}
	if c != nil {
		out.Context, out.Version, out.Nodes = c.Context, c.KubernetesVersion, c.Nodes
	}
	return out, nil
}

// Stop deletes the session's clusters. With all, every ks-* cluster goes.
func (l *Lab) Stop(ctx context.Context, all bool) error {
	unlock, err := l.Store.Lock()
	if err != nil {
		return err
	}
	defer unlock()
	st, err := l.Store.Load()
	switch {
	case errors.Is(err, session.ErrNoSession):
		if !all {
			return err
		}
	case err != nil && !all:
		return err
	case err == nil:
		if err := l.stopLocked(ctx, st); err != nil {
			return err
		}
	default:
		// A corrupt state file must not block `stop --all`.
		_ = l.Store.Clear()
	}
	if all {
		names, err := l.Backend.List(ctx)
		if err != nil {
			return err
		}
		for _, n := range names {
			ui.Step(l.Out, "Deleting orphaned cluster %s", n)
			if err := l.Backend.Delete(ctx, n); err != nil {
				return err
			}
		}
		_ = l.regenKubeconfig(ctx, nil)
	}
	return nil
}

func (l *Lab) stopLocked(ctx context.Context, st *session.State) error {
	var errs []error
	for _, c := range st.Clusters {
		ui.Step(l.Out, "Deleting cluster %s", c.Name)
		if err := l.Backend.Delete(ctx, c.Name); err != nil {
			errs = append(errs, err)
		}
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("%w (state kept so you can retry `kubestronaut stop`)", err)
	}
	if err := l.Store.Clear(); err != nil {
		return err
	}
	if err := l.regenKubeconfig(ctx, nil); err != nil {
		return err
	}
	// The session directory only holds artifacts of the deleted cluster.
	_ = os.RemoveAll(l.Store.SessionDir(st.ID))
	return nil
}

// Reset recreates the cluster and reruns setup.
func (l *Lab) Reset(ctx context.Context) error {
	unlock, err := l.Store.Lock()
	if err != nil {
		return err
	}
	defer unlock()
	st, t, s, err := l.current()
	if err != nil {
		return err
	}
	c, _ := st.Cluster(t.Cluster)
	opts := StartOptions{}
	if c != nil {
		opts.K8sVersion, opts.NodeImage = c.KubernetesVersion, c.NodeImage
	}
	hints, viewed := t.HintsUsed, t.SolutionViewed
	if err := l.stopLocked(ctx, st); err != nil {
		return err
	}
	if err := l.startLocked(ctx, s, opts); err != nil {
		return err
	}
	// Resetting must not be a way to wipe the record of help taken.
	ns, err := l.Store.Load()
	if err != nil {
		return err
	}
	ns.Tasks[0].HintsUsed, ns.Tasks[0].SolutionViewed = hints, viewed
	return l.Store.Save(ns)
}

// Grade runs the checks of the active task.
func (l *Lab) Grade(ctx context.Context) (grade.Report, error) {
	unlock, err := l.Store.Lock()
	if err != nil {
		return grade.Report{}, err
	}
	defer unlock()
	st, t, s, err := l.current()
	if err != nil {
		return grade.Report{}, err
	}
	rep, err := l.gradeTask(ctx, st, t, s)
	if err != nil {
		return rep, err
	}
	t.LastGrade = &rep
	if err := l.Store.Save(st); err != nil {
		return rep, err
	}
	if b, err := marshalReport(rep); err == nil {
		gd := filepath.Join(l.Store.SessionDir(st.ID), "grades")
		if os.MkdirAll(gd, 0o700) == nil {
			_ = os.WriteFile(filepath.Join(gd, fmt.Sprintf("%s-%09d.json", l.Now().UTC().Format("20060102T150405Z"), l.Now().Nanosecond())), b, 0o600)
		}
	}
	return rep, nil
}

func (l *Lab) gradeTask(ctx context.Context, st *session.State, t *session.Task, s *scenario.Scenario) (grade.Report, error) {
	c, ok := st.Cluster(t.Cluster)
	if !ok {
		return grade.Report{}, fmt.Errorf("cluster %s is not part of the session: run `kubestronaut stop --all` and start again", t.Cluster)
	}
	dir := l.scenarioDir(st.ID, s)
	lib := filepath.Join(l.Store.SessionDir(st.ID), "lib.sh")
	env := &check.Env{
		Kube: l.KubeFor(c.AdminKubeconfig), Backend: l.Backend, Cluster: c.Name, Dir: dir,
		ScriptEnv: l.scriptEnv(*c, dir, lib), Runner: l.Runner,
	}
	rep := (&grade.Grader{Env: env}).Grade(ctx, s)
	rep.HintsUsed, rep.SolutionViewed, rep.Elapsed = t.HintsUsed, t.SolutionViewed, l.Now().Sub(st.StartedAt)
	return rep, nil
}

// Hint returns the next hint (or all of them) and counts it.
func (l *Lab) Hint(all bool) (hints []string, shown, total int, err error) {
	unlock, err := l.Store.Lock()
	if err != nil {
		return nil, 0, 0, err
	}
	defer unlock()
	st, t, s, err := l.current()
	if err != nil {
		return nil, 0, 0, err
	}
	total = len(s.Spec.Hints)
	if total == 0 {
		return nil, 0, 0, fmt.Errorf("scenario %s has no hints", s.ID)
	}
	if all {
		t.HintsUsed = total
		hints = s.Spec.Hints
	} else {
		t.HintsUsed = min(t.HintsUsed+1, total)
		hints = []string{s.Spec.Hints[t.HintsUsed-1]}
	}
	if err := l.Store.Save(st); err != nil {
		return nil, 0, 0, err
	}
	return hints, t.HintsUsed, total, nil
}

// Solution returns the reference solution text and flags the task.
func (l *Lab) Solution() (string, error) {
	unlock, err := l.Store.Lock()
	if err != nil {
		return "", err
	}
	defer unlock()
	st, t, s, err := l.current()
	if err != nil {
		return "", err
	}
	path := s.Spec.Solution.Readme
	if path == "" {
		path = s.Spec.Solution.Script
	}
	b, err := fsReadFile(s, path)
	if err != nil {
		return "", err
	}
	t.SolutionViewed = true
	if err := l.Store.Save(st); err != nil {
		return "", err
	}
	return string(b), nil
}

// RunSolution executes the solution script against the active cluster. It is
// used by validate, not by learners.
func (l *Lab) RunSolution(ctx context.Context) error {
	st, t, s, err := l.current()
	if err != nil {
		return err
	}
	c, ok := st.Cluster(t.Cluster)
	if !ok {
		return fmt.Errorf("cluster %s is not part of the session", t.Cluster)
	}
	sdir := l.Store.SessionDir(st.ID)
	data := &scenario.TemplateData{Cluster: c.Name, Context: s.Spec.Context}
	if err := s.MaterializeSolution(sdir, data); err != nil {
		return err
	}
	dir := l.scenarioDir(st.ID, s)
	script := filepath.Join(sdir, filepath.FromSlash(s.Spec.Solution.Script))
	res, err := l.Runner.Run(ctx, execx.Cmd{
		Name: "bash", Args: []string{script}, Dir: dir, Env: l.scriptEnv(*c, dir, filepath.Join(sdir, "lib.sh")),
	})
	if err != nil {
		return fmt.Errorf("run solution: %w", err)
	}
	if l.Verbose {
		_, _ = l.Out.Write(res.Stdout)
		_, _ = l.Out.Write(res.Stderr)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("solution script exited %d:\n%s", res.ExitCode, failureOutput(res))
	}
	return nil
}

// failureOutput keeps the tails of stderr and stdout apart. Joined, a chatty
// stdout (kubeadm prints its whole preflight report there) pushed the actual
// error on stderr out of the window.
func failureOutput(res execx.Result) string {
	var parts []string
	if s := strings.TrimSpace(string(res.Stderr)); s != "" {
		parts = append(parts, "stderr:\n"+lastLines(s, 20))
	}
	if s := strings.TrimSpace(string(res.Stdout)); s != "" {
		parts = append(parts, "stdout:\n"+lastLines(s, 8))
	}
	if len(parts) == 0 {
		return "(no output)"
	}
	return strings.Join(parts, "\n")
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// Node resolves the cluster and checks the alias for ssh and shell. The
// cluster comes from KS_CLUSTER (set for scripts) or the current task.
func (l *Lab) Node(clusterOverride string) (string, error) {
	if clusterOverride != "" {
		return clusterOverride, nil
	}
	st, err := l.Store.Load()
	if err != nil {
		return "", err
	}
	if st.Current < 0 || st.Current >= len(st.Tasks) {
		return "", errors.New("state file is inconsistent (no current task): run `kubestronaut stop --all`")
	}
	return st.Tasks[st.Current].Cluster, nil
}

// SSH runs a command on a node, or opens a shell if argv is empty.
func (l *Lab) SSH(ctx context.Context, clusterOverride, alias string, argv []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	cluster, err := l.Node(clusterOverride)
	if err != nil {
		return 0, err
	}
	if len(argv) == 0 {
		return 0, l.Backend.NodeShell(ctx, cluster, alias)
	}
	res, err := l.Backend.NodeExec(ctx, cluster, alias, stdin, argv...)
	if err != nil {
		return 0, err
	}
	_, _ = stdout.Write(res.Stdout)
	_, _ = stderr.Write(res.Stderr)
	return res.ExitCode, nil
}

func firstLine(err error) string {
	return strings.SplitN(err.Error(), "\n", 2)[0]
}

// createTimeoutFor is the budget for creating a cluster with the given number
// of control planes: 5 minutes, and 2 more for each control plane after the
// first (9 minutes for an HA cluster).
func createTimeoutFor(controlPlanes int) time.Duration {
	return createTimeout + time.Duration(max(controlPlanes, 1)-1)*2*time.Minute
}
