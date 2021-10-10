package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/jacobweinstock/proxydhcp/cli"
	"github.com/peterbourgon/ff/v3/ffcli"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func Execute(ctx context.Context) error {
	rootCmd, rootConfig := cli.ProxyDHCP(ctx)
	binCmd := cli.SupportedBins(ctx)
	rootC := New(rootCmd, binCmd)

	if err := rootC.Parse(os.Args[1:]); err != nil {
		return err
	}

	rootConfig.Log = defaultLogger(rootConfig.LogLevel)

	return rootC.Run(ctx)
}

// defaultLogger is zap logr implementation.
func defaultLogger(level string) logr.Logger {
	config := zap.NewProductionConfig()
	config.OutputPaths = []string{"stdout"}
	switch level {
	case "debug":
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	default:
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}
	zapLogger, err := config.Build()
	if err != nil {
		panic(fmt.Sprintf("who watches the watchmen (%v)?", err))
	}

	return zapr.NewLogger(zapLogger)
}

func New(s ...*ffcli.Command) *ffcli.Command {
	appName := "proxydhcp"

	fs := flag.NewFlagSet(appName, flag.ExitOnError)

	return &ffcli.Command{
		ShortUsage: "proxydhcp <subcommand>",
		FlagSet:    fs,
		/*Options: []ff.Option{
			ff.WithEnvVarPrefix(strings.ToUpper(appName)),
			ff.WithConfigFileFlag("config"),
			ff.WithAllowMissingConfigFile(true),
			ff.WithIgnoreUndefined(true),
		},*/
		Exec: func(_ context.Context, _ []string) error {
			return flag.ErrHelp
		},
		Subcommands: s,
	}
}
