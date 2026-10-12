// Package session persists the active lab session under the state directory.
package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/JoshSLawrence/kubestronaut/internal/backend"
	"github.com/JoshSLawrence/kubestronaut/internal/grade"
)

// SchemaVersion is the current state.json version.
const SchemaVersion = 1

// Modes of a session.
const (
	ModeScenario = "scenario"
	ModeExam     = "exam"
)

// ErrNoSession means no session is active.
var ErrNoSession = errors.New("no active session: run `kubestronaut start <id>` first")

// State is the content of state.json.
type State struct {
	Version   int          `json:"version"`
	Mode      string       `json:"mode"`
	ID        string       `json:"id"`
	Exam      string       `json:"exam,omitempty"`
	StartedAt time.Time    `json:"startedAt"`
	Deadline  *time.Time   `json:"deadline,omitempty"`
	Clusters  []ClusterRef `json:"clusters"`
	Tasks     []Task       `json:"tasks"`
	Current   int          `json:"current"`
}

// ClusterRef is one cluster of a session.
type ClusterRef struct {
	Name              string         `json:"name"`
	Context           string         `json:"context"`
	AdminKubeconfig   string         `json:"adminKubeconfig"`
	KubernetesVersion string         `json:"kubernetesVersion"`
	Nodes             []backend.Node `json:"nodes"`
}

// Task is one scenario in a session.
type Task struct {
	Scenario       string        `json:"scenario"`
	Cluster        string        `json:"cluster"`
	Weight         int           `json:"weight"`
	HintsUsed      int           `json:"hintsUsed"`
	SolutionViewed bool          `json:"solutionViewed"`
	LastGrade      *grade.Report `json:"lastGrade,omitempty"`
}

// Cluster finds a cluster by name.
func (s *State) Cluster(name string) (*ClusterRef, bool) {
	for i := range s.Clusters {
		if s.Clusters[i].Name == name {
			return &s.Clusters[i], true
		}
	}
	return nil, false
}

// NewID returns a time-sortable unique id.
func NewID(now time.Time) string {
	var b [3]byte
	_, _ = rand.Read(b[:])
	return now.UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(b[:])
}

// Store reads and writes state under a home directory.
type Store struct {
	Root string
}

// Paths inside the home directory.
func (s *Store) statePath() string { return filepath.Join(s.Root, "state.json") }
func (s *Store) lockPath() string  { return filepath.Join(s.Root, "lock") }

// KubeconfigPath is the user-facing merged kubeconfig.
func (s *Store) KubeconfigPath() string { return filepath.Join(s.Root, "kubeconfig") }

// SessionDir is the working directory of one session.
func (s *Store) SessionDir(id string) string { return filepath.Join(s.Root, "sessions", id) }

// ScratchKubeconfig is a throwaway path handed to tools that insist on
// editing a kubeconfig, so the user's own files stay untouched.
func (s *Store) ScratchKubeconfig() string { return filepath.Join(s.Root, "scratch.kubeconfig") }

// Init makes sure the home directory exists and is private. Only a directory
// created here is ever chmodded: an existing one (a user's own --home such as
// a shared temp dir) is validated and refused if unsafe, never modified.
func (s *Store) Init() error {
	fi, err := os.Lstat(s.Root)
	if errors.Is(err, os.ErrNotExist) {
		if err := s.refuseSystemDir(); err != nil {
			return err
		}
		if err := os.MkdirAll(s.Root, 0o700); err != nil {
			return fmt.Errorf("create state dir %s: %w", s.Root, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("state dir %s: %w", s.Root, err)
	}
	if err := s.refuseSystemDir(); err != nil {
		return err
	}
	switch {
	case fi.Mode()&os.ModeSymlink != 0:
		return fmt.Errorf("state dir %s is a symlink: choose a real, dedicated directory", s.Root)
	case !fi.IsDir():
		return fmt.Errorf("state dir %s is not a directory: choose another --home", s.Root)
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Getuid() {
		return fmt.Errorf("state dir %s is owned by another user: choose a directory you own", s.Root)
	}
	if fi.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("state dir %s is group/world accessible: run `chmod 700 %s` or choose a dedicated directory", s.Root, s.Root)
	}
	return nil
}

// refuseSystemDir rejects directories that are shared by design.
func (s *Store) refuseSystemDir() error {
	root := filepath.Clean(s.Root)
	forbidden := []string{"/", os.TempDir()}
	if h, err := os.UserHomeDir(); err == nil {
		forbidden = append(forbidden, h)
	}
	for _, f := range forbidden {
		if root == filepath.Clean(f) {
			return fmt.Errorf("refusing to use %s as the state dir: choose a dedicated directory such as ~/.kubestronaut", s.Root)
		}
	}
	return nil
}

// Lock takes an exclusive lock so two commands cannot change state at once.
// The returned function releases it.
func (s *Store) Lock() (func(), error) {
	if err := s.Init(); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(s.lockPath(), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("another kubestronaut command is running (lock %s held): wait for it to finish and retry: %w", s.lockPath(), err)
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}

// Load reads the state; it returns ErrNoSession if none exists.
func (s *Store) Load() (*State, error) {
	b, err := os.ReadFile(s.statePath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNoSession
	}
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}
	var st State
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, fmt.Errorf("state file %s is corrupt (%w): run `kubestronaut stop --all` to start over", s.statePath(), err)
	}
	if st.Version != SchemaVersion {
		return nil, fmt.Errorf("state file %s has version %d but this build understands %d: run `kubestronaut stop --all` with the build that created it, then retry",
			s.statePath(), st.Version, SchemaVersion)
	}
	return &st, nil
}

// Save writes the state atomically: a temp file in the same directory is
// renamed over the target so a crash never leaves half a file.
func (s *Store) Save(st *State) error {
	if err := s.Init(); err != nil {
		return err
	}
	st.Version = SchemaVersion
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.Root, "state-*.tmp")
	if err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write state: %w", err)
	}
	// Sync first so a crash cannot leave a renamed but empty file.
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	if err := os.Rename(tmp.Name(), s.statePath()); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	return nil
}

// Clear removes the state file; a missing file is fine.
func (s *Store) Clear() error {
	if err := os.Remove(s.statePath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
