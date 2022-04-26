// Package cli implements the functionality for running proxydhcp as a CLI.
package cli

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	"github.com/go-playground/validator/v10"
	"github.com/hashicorp/go-multierror"
	"github.com/jacobweinstock/proxydhcp/proxy"
	"github.com/peterbourgon/ff/v3/ffcli"
	"golang.org/x/sync/errgroup"
	"inet.af/netaddr"
)

const appName = "proxy"

// Config is the configuration for the proxydhcp command.
type Config struct {
	LogLevel        string `vname:"-loglevel" validate:"oneof=debug info"`
	TFTPAddr        string `vname:"-remote-tftp" validate:"required,hostname_port"`
	HTTPAddr        string `vname:"-remote-http" validate:"required,hostname_port"`
	IPXEAddr        string `vname:"-remote-ipxe" validate:"required,url"`
	IPXEScript      string `vname:"-ipxe-script" validate:"required"`
	ProxyAddr       string `vname:"-proxy-addr" validate:"required,ip"`
	CustomUserClass string
	Log             logr.Logger
	Authz           proxy.Allower
}

// ProxyDHCP returns the CLI command and Config struct for the proxydhcp command.
func ProxyDHCP(_ context.Context) (*ffcli.Command, *Config) {
	fs := flag.NewFlagSet(appName, flag.ExitOnError)
	cfg := &Config{
		Log:       logr.Discard(),
		ProxyAddr: "0.0.0.0:67",
		Authz:     proxy.AllowAll{},
	}

	RegisterFlags(cfg, fs)

	return &ffcli.Command{
		Name:        appName,
		ShortUsage:  fmt.Sprintf("%v runs the proxyDHCP server", appName),
		FlagSet:     fs,
		Exec:        cfg.exec,
		Subcommands: []*ffcli.Command{File(), Tink()},
	}, cfg
}

// RegisterFlags registers CLI flags for the proxydhcp comand.
func RegisterFlags(c *Config, fs *flag.FlagSet) {
	fs.StringVar(&c.LogLevel, "loglevel", "info", "log level (optional)")
	fs.StringVar(&c.ProxyAddr, "proxy-addr", "0.0.0.0", "IP associated to the network interface to listen on for proxydhcp requests.")
	fs.StringVar(&c.TFTPAddr, "remote-tftp", "", "IP and URI of the TFTP server providing iPXE binaries (192.168.2.5:69).")
	fs.StringVar(&c.HTTPAddr, "remote-http", "", "IP, port, and URI of the HTTP server providing iPXE binaries (i.e. 192.168.2.4:80).")
	fs.StringVar(&c.IPXEAddr, "remote-ipxe", "", "A url where an iPXE script is served (i.e. http://192.168.2.3:8080).")
	fs.StringVar(&c.IPXEScript, "remote-ipxe-script", "auto.ipxe", "The name of the iPXE script to use. used with remote-ipxe (http://192.168.2.3/<mac-addr>/auto.ipxe)")
	fs.StringVar(&c.CustomUserClass, "user-class", "", "A custom user-class (dhcp option 77) to use to determine when to pivot to serving the ipxe script from the ipxe-url flag.")
}

// validateConfig validates the config struct based on its struct tags.
func (c *Config) validateConfig() error {
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
		// s := "'%v' is not a valid for flag %v\n"
		for _, msg := range err.(validator.ValidationErrors) {
			errMsg = append(errMsg, fmt.Sprintf("%v '%v' not valid: '%v'", msg.Field(), msg.Value(), msg.Tag()))
		}
		errMsg = append(errMsg, "\n")
		return fmt.Errorf("%v: %w", strings.Join(errMsg, "\n"), flag.ErrHelp)
	}

	return nil
}

// exec function for this command.
func (c *Config) exec(ctx context.Context, args []string) error {
	if err := c.validateConfig(); err != nil {
		return err
	}

	return c.run(ctx, args)
}

// run the proxyDHCP server.
func (c *Config) run(ctx context.Context, _ []string) error {
	ta, err := netaddr.ParseIPPort(c.TFTPAddr)
	if err != nil {
		return err
	}
	ha, err := netaddr.ParseIPPort(c.HTTPAddr)
	if err != nil {
		return err
	}
	ia, err := url.Parse(c.IPXEAddr)
	if err != nil {
		return err
	}
	opts := []proxy.Option{
		proxy.WithLogger(c.Log),
		proxy.WithAllower(c.Authz),
		proxy.WithIPXEScript(c.IPXEScript),
		proxy.WithUserClass(c.CustomUserClass),
	}
	h := proxy.NewHandler(ctx, ta, ha, ia, opts...)

	u, err := netaddr.ParseIPPort(c.ProxyAddr + ":67")
	if err != nil {
		return err
	}
	rs, err := proxy.Server(ctx, u, nil, h.Handle)
	if err != nil {
		return err
	}

	h2 := proxy.NewHandler(ctx, ta, ha, ia, opts...)
	bs, err := proxy.Server(ctx, u.WithPort(4011), nil, h2.Handle)
	if err != nil {
		return err
	}

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		h.Log.Info("starting proxydhcp", "addr1", c.ProxyAddr, "addr2", "0.0.0.0:67")
		return rs.Serve()
	})
	g.Go(func() error {
		h.Log.Info("starting proxydhcp", "addr1", c.ProxyAddr, "addr2", "0.0.0.0:4011")
		return bs.Serve()
	})

	errCh := make(chan error)
	go func() {
		errCh <- g.Wait()
	}()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		h.Log.Info("shutting down")
		return multierror.Append(nil, rs.Close(), bs.Close()).ErrorOrNil()
	}
}
