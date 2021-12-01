package cli

import (
	"context"
	"flag"

	"github.com/imdario/mergo"
	"github.com/jacobweinstock/proxydhcp/authz/tink"
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/tinkerbell/tink/protos/hardware"
)

const tinkCLI = "tink"

// TinkCfg is the configuration for the tink backend.
type TinkCfg struct {
	Config
	// TLS can be one of the following
	// 1. location on disk of a cert
	// example: /location/on/disk/of/cert
	// 2. URL from which to GET a cert
	// example: http://weburl:8080/cert
	// 3. boolean; true if the tink server (specified by the Tink key/value) has a cert from a known CA
	// false if the tink server does not have TLS enabled
	// example: true
	TLS string
	// Tink is the URL:Port for the tink server
	Tink string `validate:"required"`
}

// Tink is the subcommand that communicates with Tink server for authorizing PXE boot requests.
func Tink() *ffcli.Command {
	cfg := &TinkCfg{}
	fs := flag.NewFlagSet(tinkCLI, flag.ExitOnError)
	RegisterFlagsTink(cfg, fs)

	return &ffcli.Command{
		Name:       tinkCLI,
		ShortUsage: tinkCLI,
		FlagSet:    fs,
		Exec: func(ctx context.Context, _ []string) error {
			return cfg.Exec(ctx, nil)
		},
	}
}

// RegisterFlagsTink registers the flags for the tink subcommand.
func RegisterFlagsTink(cfg *TinkCfg, fs *flag.FlagSet) {
	RegisterFlags(&cfg.Config, fs)
	fs.StringVar(&cfg.Tink, "tink", "", "tink server URL (required)")
	description := "(file:///path/to/cert/tink.cert, http://tink-server:42114/cert, boolean (false - no TLS, true - tink has a cert from known CA) (optional)"
	fs.StringVar(&cfg.TLS, "tls", "false", "tink server TLS "+description)
}

// Exec is the execution function for the tink subcommand.
func (t *TinkCfg) Exec(ctx context.Context, _ []string) error {
	defaults := TinkCfg{
		Config: Config{
			LogLevel: "info",
			Log:      defaultLogger("info"),
		},
	}
	err := mergo.Merge(t, defaults, mergo.WithTransformers(logger{}))
	if err != nil {
		return err
	}
	t.Log.Info("starting ipxe", "tftp-addr", t.TFTPAddr, "http-addr", t.HTTPAddr)
	gc, err := tink.SetupClient(ctx, t.Log, t.TLS, t.Tink)
	if err != nil {
		return err
	}
	c := hardware.NewHardwareServiceClient(gc)
	tb := &tink.Tinkerbell{Client: c, Log: t.Log}
	t.Config.Authz = tb
	return t.Config.run(ctx, nil)
}
