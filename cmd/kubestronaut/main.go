// Command kubestronaut runs Kubernetes certification lab scenarios on kind.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/JoshSLawrence/kubestronaut/internal/cli"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	go func() {
		// After the first signal restore the default behaviour, so a second
		// Ctrl-C during cleanup terminates the process at once.
		<-ctx.Done()
		stop()
	}()
	code := cli.Execute(ctx, &cli.App{Version: version}, os.Args[1:], os.Stdout, os.Stderr, os.Stdin)
	stop()
	os.Exit(code)
}
