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
	code := cli.Execute(ctx, &cli.App{Version: version}, os.Args[1:], os.Stdout, os.Stderr, os.Stdin)
	stop()
	os.Exit(code)
}
