package cli

import (
	"context"
	"flag"
	"fmt"
	"net"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	"github.com/go-playground/validator/v10"
	"github.com/jacobweinstock/proxydhcp/proxy"
	"github.com/peterbourgon/ff/v3"
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/pkg/errors"
)

const appName = "proxy"

type Config struct {
	Command         ffcli.Command
	LogLevel        string `vname:"-loglevel" validate:"oneof=debug info"`
	TftpAddr        string `vname:"-tftp-addr" validate:"required,url"`
	HttpAddr        string `vname:"-http-addr" validate:"required,url"`
	IPXEURL         string `vname:"-ipxe-url" validate:"required,url"`
	Addr            string `vname:"-addr" validate:"hostname_port"`
	CustomUserClass string
	Log             logr.Logger
}

// Option for setting optional Client values
type ProxyOption func(*Config)

func ProxyWithName(name string) ProxyOption {
	return func(cfg *Config) {
		cfg.Command.Name = name
	}
}

func ProxyWithShortUsage(shortUsage string) ProxyOption {
	return func(cfg *Config) {
		cfg.Command.ShortUsage = shortUsage
	}
}

func ProxyWithUsageFunc(usageFunc func(*ffcli.Command) string) ProxyOption {
	return func(cfg *Config) {
		cfg.Command.UsageFunc = usageFunc
	}
}

func ProxyWithFlagSet(flagSet *flag.FlagSet) ProxyOption {
	return func(cfg *Config) {
		cfg.Command.FlagSet = flagSet
	}
}

func ProxyWithOptions(opts ...ff.Option) ProxyOption {
	return func(cfg *Config) {
		cfg.Command.Options = append(cfg.Command.Options, opts...)
	}
}

func ProxyWithLogger(l logr.Logger) ProxyOption {
	return func(cfg *Config) {
		cfg.Log = l
	}
}

func ProxyDHCP(ctx context.Context, opts ...ProxyOption) (*ffcli.Command, *Config) {
	fs := flag.NewFlagSet(appName, flag.ExitOnError)
	cfg := &Config{
		Log:  logr.Discard(),
		Addr: "0.0.0.0:67",
		Command: ffcli.Command{
			Name:       appName,
			ShortUsage: fmt.Sprintf("%v runs the proxyDHCP server", appName),
			FlagSet:    fs,
		},
	}

	RegisterFlags(cfg, fs)
	for _, opt := range opts {
		opt(cfg)
	}

	return &ffcli.Command{
		Name:        cfg.Command.Name,
		ShortUsage:  cfg.Command.ShortHelp,
		ShortHelp:   cfg.Command.ShortHelp,
		LongHelp:    cfg.Command.LongHelp,
		UsageFunc:   cfg.Command.UsageFunc,
		FlagSet:     cfg.Command.FlagSet,
		Options:     cfg.Command.Options,
		Subcommands: cfg.Command.Subcommands,
		Exec:        cfg.Exec,
	}, cfg
}

func RegisterFlags(c *Config, fs *flag.FlagSet) {
	fs.StringVar(&c.LogLevel, "loglevel", "info", "log level (optional)")
	fs.StringVar(&c.Addr, "addr", "0.0.0.0:67", "IP and port to listen on for proxydhcp requests.")
	fs.StringVar(&c.TftpAddr, "tftp-addr", "", "IP and URI of the TFTP server providing iPXE binaries (192.168.2.5/binaries).")
	fs.StringVar(&c.HttpAddr, "http-addr", "", "IP, port, and URI of the HTTP server providing iPXE binaries (i.e. 192.168.2.4:8080/binaries).")
	fs.StringVar(&c.IPXEURL, "ipxe-url", "", "A full url to an iPXE script (i.e. http://192.168.2.3/auto.ipxe).")
	fs.StringVar(&c.CustomUserClass, "user-class", "", "A custom user-class (dhcp option 77) to use to determine when to pivot to serving the ipxe script from the ipxe-url flag.")
}

func (c *Config) ValidateConfig() error {
	v := validator.New()
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("vname"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
	if err := v.Struct(c); err != nil {
		var errMsg []string
		//s := "'%v' is not a valid for flag %v\n"
		for _, msg := range err.(validator.ValidationErrors) {
			errMsg = append(errMsg, fmt.Sprintf("%v '%v' not valid: '%v'", msg.Field(), msg.Value(), msg.Tag()))
		}
		errMsg = append(errMsg, "\n")
		return errors.Wrap(flag.ErrHelp, strings.Join(errMsg, "\n"))
	}

	return nil
}

// Exec function for this command.
func (c *Config) Exec(ctx context.Context, args []string) error {
	if err := c.ValidateConfig(); err != nil {

		return err
	}

	return c.Run(ctx, args)
}

func (c *Config) Run(ctx context.Context, _ []string) error {
	redirectionListener, err := proxy.NewListener(c.Addr)
	if err != nil {
		return err
	}
	defer redirectionListener.Close()
	log := c.Log

	go func() {
		<-ctx.Done()
		redirectionListener.Close()
		log.V(0).Info("shutting down proxydhcp", "addr", c.Addr)
	}()

	bootListener, err := net.ListenPacket("udp4", fmt.Sprintf("%s:%d", "0.0.0.0", 4011))
	if err != nil {
		return err
	}
	defer bootListener.Close()
	go func() {
		<-ctx.Done()
		bootListener.Close()
		log.V(0).Info("shutting down proxydhcp", "addr", c.Addr)
	}()
	go proxy.ServeBoot(ctx, log, bootListener, c.TftpAddr, c.HttpAddr, c.IPXEURL, c.CustomUserClass)

	log.V(0).Info("starting proxydhcp", "addr1", c.Addr, "addr2", "0.0.0.0:4011")
	// proxy.Serve will block until the context (ctx) is canceled .
	proxy.Serve(ctx, log, redirectionListener, c.TftpAddr, c.HttpAddr, c.IPXEURL, c.CustomUserClass)
	return nil
}
