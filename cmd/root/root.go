package root

import (
	"context"
	"flag"
	"strings"

	"github.com/go-logr/logr"
	"github.com/peterbourgon/ff/v3"
	"github.com/peterbourgon/ff/v3/ffcli"
)

const appName = "proxydhcp"

type Config struct {
	LogLevel string
	Log      logr.Logger
	Addr     string
}

func New() (*ffcli.Command, *Config) {
	var cfg Config

	fs := flag.NewFlagSet(appName, flag.ExitOnError)
	cfg.RegisterFlags(fs)

	return &ffcli.Command{
		ShortUsage: "proxydhcp [flags] <subcommand>",
		FlagSet:    fs,
		Options: []ff.Option{
			ff.WithEnvVarPrefix(strings.ToUpper(appName)),
			ff.WithAllowMissingConfigFile(true),
			ff.WithIgnoreUndefined(true),
		},
		Exec: cfg.Exec,
	}, &cfg
}

func (c *Config) RegisterFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.LogLevel, "loglevel", "info", "log level (optional)")
	fs.StringVar(&c.Addr, "addr", "0.0.0.0:67", "IP and port to listen on for proxydhcp requests.")
}

// Exec function for this command.
func (c *Config) Exec(context.Context, []string) error {
	// The root command has no meaning, so if it gets executed,
	// display the usage text to the user instead.
	return flag.ErrHelp
}
