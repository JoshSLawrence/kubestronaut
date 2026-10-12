// Package scenario loads, templates and statically validates scenario packs.
package scenario

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"time"

	"github.com/JoshSLawrence/kubestronaut/internal/kube"
)

// APIVersion is the only schema version understood.
const APIVersion = "kubestronaut/v1alpha1"

// Kinds of document.
const (
	KindPack     = "Pack"
	KindScenario = "Scenario"
	KindExam     = "Exam"
)

// Defaults applied when a field is omitted.
const (
	DefaultCheckTimeout  = 20 * time.Second
	DefaultScriptTimeout = 3 * time.Minute
	DefaultWaitTimeout   = 2 * time.Minute
	DefaultPassPercent   = 100
)

// Duration is a time.Duration that reads and writes Go duration strings.
type Duration time.Duration

// D converts to time.Duration.
func (d Duration) D() time.Duration { return time.Duration(d) }

// UnmarshalJSON implements json.Unmarshaler.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("duration must be a string such as \"30s\": %w", err)
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	*d = Duration(v)
	return nil
}

// MarshalJSON implements json.Marshaler.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// Pack groups scenarios for one certification.
type Pack struct {
	APIVersion string   `json:"apiVersion"`
	Kind       string   `json:"kind"`
	Metadata   PackMeta `json:"metadata"`
	Spec       PackSpec `json:"spec"`
}

// PackMeta is a pack's identity.
type PackMeta struct {
	Name string `json:"name"`
}

// PackSpec holds pack-wide settings.
type PackSpec struct {
	Title       string   `json:"title"`
	PassPercent int      `json:"passPercent"`
	Domains     []Domain `json:"domains"`
}

// Domain is one curriculum domain.
type Domain struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Weight int    `json:"weight"`
}

// Scenario is one lab.
type Scenario struct {
	APIVersion string   `json:"apiVersion"`
	Kind       string   `json:"kind"`
	Metadata   Metadata `json:"metadata"`
	Spec       Spec     `json:"spec"`

	// Fields below are filled by the loader.
	ID      string `json:"-"` // pack/name
	Pack    string `json:"-"`
	DirName string `json:"-"`
	Prompt  string `json:"-"` // rendered prompt file
	// Files is rooted at the scenario directory.
	Files fs.FS `json:"-"`
	// UsedNodes lists the aliases referenced by {{node "..."}} while rendering.
	UsedNodes []string `json:"-"`
}

// Metadata describes a scenario.
type Metadata struct {
	Name         string   `json:"name"`
	Title        string   `json:"title"`
	Domain       string   `json:"domain"`
	Difficulty   string   `json:"difficulty,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	Experimental bool     `json:"experimental,omitempty"`
}

// Spec is the behaviour of a scenario.
type Spec struct {
	TimeLimit   Duration    `json:"timeLimit,omitempty"`
	Weight      int         `json:"weight,omitempty"`
	PassPercent int         `json:"passPercent,omitempty"`
	Context     string      `json:"context"`
	Namespaces  []string    `json:"namespaces,omitempty"`
	Exclusive   bool        `json:"exclusive,omitempty"`
	Cluster     ClusterConf `json:"cluster,omitempty"`
	Images      []string    `json:"images,omitempty"`
	PromptFile  string      `json:"prompt"`
	Hints       []string    `json:"hints,omitempty"`
	Setup       []SetupStep `json:"setup,omitempty"`
	Checks      []CheckSpec `json:"checks"`
	Solution    Solution    `json:"solution"`
	Validation  Validation  `json:"validation,omitempty"`
}

// ClusterConf sizes the cluster a scenario needs.
type ClusterConf struct {
	Workers           int      `json:"workers,omitempty"`
	KubernetesVersion string   `json:"kubernetesVersion,omitempty"`
	KindConfigPatches []string `json:"kindConfigPatches,omitempty"`
	// Networking passes the few kind networking options scenarios need.
	Networking NetworkingConf `json:"networking,omitempty"`
}

// NetworkingConf selects kind networking options.
type NetworkingConf struct {
	// DisableDefaultCNI creates the cluster without kind's CNI (kindnet), so
	// nodes stay NotReady until the learner installs one.
	DisableDefaultCNI bool `json:"disableDefaultCNI,omitempty"`
}

// SetupStep is one ordered setup action; exactly one of Script, Manifest or
// Wait is set.
type SetupStep struct {
	Script   string   `json:"script,omitempty"`
	Manifest string   `json:"manifest,omitempty"`
	Wait     *Wait    `json:"wait,omitempty"`
	Timeout  Duration `json:"timeout,omitempty"`
}

// Wait blocks until a condition on a resource holds.
type Wait struct {
	Ref     kube.Ref `json:"ref"`
	For     string   `json:"for"`
	Timeout Duration `json:"timeout,omitempty"`
}

// Solution points at the reference solution.
type Solution struct {
	Script string `json:"script"`
	Readme string `json:"readme,omitempty"`
}

// Validation tunes validate --e2e.
type Validation struct {
	PassBeforeSolution []string `json:"passBeforeSolution,omitempty"`
	Timeout            Duration `json:"timeout,omitempty"`
}

// Retry repeats a check until it passes.
type Retry struct {
	Attempts int      `json:"attempts"`
	Interval Duration `json:"interval,omitempty"`
}

// CheckBase holds the fields every check shares.
type CheckBase struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Weight      int      `json:"weight,omitempty"`
	Timeout     Duration `json:"timeout,omitempty"`
	Retry       Retry    `json:"retry,omitempty"`
	Type        string   `json:"type"`
}

// CheckSpec is a check as written in YAML. Type-specific fields stay in Raw
// and are decoded strictly by the check package.
type CheckSpec struct {
	CheckBase
	Raw json.RawMessage `json:"-"`
}

// BaseKeys are the JSON keys that belong to CheckBase.
var BaseKeys = []string{"id", "description", "weight", "timeout", "retry", "type"}

// UnmarshalJSON implements json.Unmarshaler.
func (c *CheckSpec) UnmarshalJSON(b []byte) error {
	c.Raw = append(c.Raw[:0], b...)
	return json.Unmarshal(b, &c.CheckBase)
}

// Exam types only; the exam logic arrives with exam mode.

// Exam is a timed multi-task exam definition.
type Exam struct {
	APIVersion string   `json:"apiVersion"`
	Kind       string   `json:"kind"`
	Metadata   ExamMeta `json:"metadata"`
	Spec       ExamSpec `json:"spec"`
}

// ExamMeta is an exam's identity.
type ExamMeta struct {
	Name string `json:"name"`
}

// ExamSpec holds an exam's settings.
type ExamSpec struct {
	Title       string        `json:"title"`
	Duration    Duration      `json:"duration"`
	PassPercent int           `json:"passPercent"`
	AllowGrade  bool          `json:"allowGrade,omitempty"`
	MaxNodes    int           `json:"maxNodes,omitempty"`
	Clusters    []ExamCluster `json:"clusters"`
	Tasks       []ExamTask    `json:"tasks"`
}

// ExamCluster is a cluster an exam creates.
type ExamCluster struct {
	Name    string `json:"name"`
	Context string `json:"context"`
	Workers int    `json:"workers,omitempty"`
}

// ExamTask places a scenario on an exam cluster.
type ExamTask struct {
	Scenario string `json:"scenario"`
	Cluster  string `json:"cluster"`
	Weight   int    `json:"weight"`
}
