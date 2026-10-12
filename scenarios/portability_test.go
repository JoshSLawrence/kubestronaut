package scenarios

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Scenario scripts and lib.sh run on the learner's machine, which may be a Mac
// whose /bin/bash is 3.2. Constructs from bash 4 or later fail there, often
// only when a particular check runs.
var bash4 = map[string]*regexp.Regexp{
	"mapfile":              regexp.MustCompile(`\bmapfile\b`),
	"readarray":            regexp.MustCompile(`\breadarray\b`),
	"declare -A":           regexp.MustCompile(`\b(declare|local|typeset)\s+-[a-zA-Z]*A`),
	"declare -n":           regexp.MustCompile(`\b(declare|local|typeset)\s+-[a-zA-Z]*n\b`),
	"case conversion":      regexp.MustCompile(`\$\{[^}]*(,,|\^\^)`),
	"|&":                   regexp.MustCompile(`\|&`),
	"[[ -v":                regexp.MustCompile(`\[\[\s+-v\s`),
	"${var@op}":            regexp.MustCompile(`\$\{[A-Za-z_][A-Za-z_0-9]*@[QEPAa]\}`),
	"&>>":                  regexp.MustCompile(`&>>`),
	"coproc":               regexp.MustCompile(`\bcoproc\b`),
	"wait -n":              regexp.MustCompile(`\bwait\s+-n\b`),
	"negative array index": regexp.MustCompile(`\[-[0-9]+\]\}`),
}

func scriptFiles(t *testing.T) []string {
	t.Helper()
	files := []string{filepath.Join("..", "internal", "setup", "lib.sh")}
	err := filepath.WalkDir(".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".sh") {
			files = append(files, p)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestScriptsAvoidBash4(t *testing.T) {
	files := scriptFiles(t)
	if len(files) < 10 {
		t.Fatalf("found only %d scripts: is the test looking in the right place?", len(files))
	}
	for _, f := range files {
		b, err := os.ReadFile(filepath.Clean(f))
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				continue
			}
			for name, re := range bash4 {
				if re.MatchString(line) {
					t.Errorf("%s:%d uses %s, which bash 3.2 lacks: %s", f, i+1, name, strings.TrimSpace(line))
				}
			}
		}
	}
}

// TestScriptsParseWithSystemBash parses every script with /bin/bash when that
// is the old bash a Mac ships, which catches syntax the patterns above miss.
func TestScriptsParseWithSystemBash(t *testing.T) {
	out, err := exec.Command("/bin/bash", "-c", "echo ${BASH_VERSINFO[0]}").Output()
	if err != nil || strings.TrimSpace(string(out)) != "3" {
		t.Skip("/bin/bash is not bash 3.x")
	}
	for _, f := range scriptFiles(t) {
		cmd := exec.Command("/bin/bash", "-c", `/bin/bash -n "$KS_SCRIPT"`)
		cmd.Env = append(os.Environ(), "KS_SCRIPT="+f)
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("/bin/bash -n %s: %v\n%s", f, err, b)
		}
	}
}
