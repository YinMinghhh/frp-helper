package main

import (
	"context"
	"fmt"
	"os"

	"frp-helper/internal/app"
	"frp-helper/internal/cli"
)

func main() {
	ctx := context.Background()

	application, err := app.New(os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	runner := cli.New(application, os.Stdout, os.Stderr)
	if err := runner.Run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
