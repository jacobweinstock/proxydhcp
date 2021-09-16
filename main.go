package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jacobweinstock/proxydhcp/cmd"
)

func main() {
	exitCode := 0
	defer func() {
		os.Exit(exitCode)
	}()

	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGHUP, syscall.SIGTERM)
	defer done()

	if err := cmd.Execute(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		exitCode = 1
	}
}
