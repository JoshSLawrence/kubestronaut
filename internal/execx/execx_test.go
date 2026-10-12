package execx

import (
	"context"
	"strings"
	"testing"
)

func TestOSRun(t *testing.T) {
	res, err := OS{}.Run(context.Background(), Cmd{
		Name: "sh", Args: []string{"-c", "echo out; echo err >&2; exit 3"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 3 || string(res.Stdout) != "out\n" || string(res.Stderr) != "err\n" {
		t.Fatalf("unexpected result %+v", res)
	}
}

func TestOSRunStdinEnv(t *testing.T) {
	res, err := OS{}.Run(context.Background(), Cmd{
		Name: "sh", Args: []string{"-c", "cat; echo $KS_X"},
		Env: []string{"KS_X=v"}, Stdin: strings.NewReader("in\n"),
	})
	if err != nil || string(res.Stdout) != "in\nv\n" {
		t.Fatalf("got %q err %v", res.Stdout, err)
	}
}

func TestOSRunNotFound(t *testing.T) {
	if _, err := (OS{}).Run(context.Background(), Cmd{Name: "no-such-binary-ks"}); err == nil {
		t.Fatal("want error for missing binary")
	}
}

func TestOSRunCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (OS{}).Run(ctx, Cmd{Name: "sleep", Args: []string{"5"}}); err == nil {
		t.Fatal("want error on cancelled context")
	}
}

func TestFakeMatching(t *testing.T) {
	f := &Fake{}
	f.Respond([]string{"kubectl"}, Result{Stdout: []byte("any")}, nil)
	f.Respond([]string{"kubectl", "get"}, Result{Stdout: []byte("get")}, nil)
	f.Once([]string{"docker", "ps"}, Result{Stdout: []byte("first")}, nil)
	f.Once([]string{"docker", "ps"}, Result{Stdout: []byte("second")}, nil)

	r, _ := f.Run(context.Background(), Cmd{Name: "kubectl", Args: []string{"get", "po"}})
	if string(r.Stdout) != "get" {
		t.Fatalf("want most specific later rule, got %q", r.Stdout)
	}
	r, _ = f.Run(context.Background(), Cmd{Name: "kubectl", Args: []string{"apply"}})
	if string(r.Stdout) != "any" {
		t.Fatalf("got %q", r.Stdout)
	}
	for _, want := range []string{"first", "second", ""} {
		r, _ = f.Run(context.Background(), Cmd{Name: "docker", Args: []string{"ps"}})
		if string(r.Stdout) != want {
			t.Fatalf("want %q got %q", want, r.Stdout)
		}
	}
	if err := f.Interactive(context.Background(), Cmd{Name: "x"}); err != nil {
		t.Fatal(err)
	}
	if got := f.Lines()[0]; got != "kubectl get po" {
		t.Fatalf("lines: %q", got)
	}
}
