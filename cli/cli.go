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
	"github.com/jacobweinstock/proxydhcp/proxy"
	"github.com/peterbourgon/ff/v3/ffcli"
	"golang.org/x/sync/errgroup"
	"inet.af/netaddr"
)

const appName = "proxy"

type Config struct {
	LogLevel        string `vname:"-loglevel" validate:"oneof=debug info"`
	TFTPAddr        string `vname:"-remote-tftp" validate:"required,hostname_port"`
	HTTPAddr        string `vname:"-remote-http" validate:"required,hostname_port"`
	IPXEAddr        string `vname:"-remote-ipxe" validate:"required,url"`
	IPXEScript      string `vname:"-ipxe-script" validate:"required"`
	ProxyAddr       string `vname:"-proxy-addr" validate:"required,ip"`
	CustomUserClass string
	Log             logr.Logger
}

func ProxyDHCP(_ context.Context) (*ffcli.Command, *Config) {
	fs := flag.NewFlagSet(appName, flag.ExitOnError)
	cfg := &Config{
		Log:       logr.Discard(),
		ProxyAddr: "0.0.0.0:67",
	}

	RegisterFlags(cfg, fs)

	return &ffcli.Command{
		Name:       appName,
		ShortUsage: fmt.Sprintf("%v runs the proxyDHCP server", appName),
		FlagSet:    fs,
		Exec:       cfg.exec,
	}, cfg
}

func RegisterFlags(c *Config, fs *flag.FlagSet) {
	fs.StringVar(&c.LogLevel, "loglevel", "info", "log level (optional)")
	fs.StringVar(&c.ProxyAddr, "proxy-addr", "0.0.0.0:67", "IP and port to listen on for proxydhcp requests.")
	fs.StringVar(&c.TFTPAddr, "remote-tftp", "", "IP and URI of the TFTP server providing iPXE binaries (192.168.2.5/binaries).")
	fs.StringVar(&c.HTTPAddr, "remote-http", "", "IP, port, and URI of the HTTP server providing iPXE binaries (i.e. 192.168.2.4:8080/binaries).")
	fs.StringVar(&c.IPXEAddr, "remote-ipxe", "", "A url where an iPXE script is served (i.e. http://192.168.2.3).")
	fs.StringVar(&c.IPXEScript, "remote-ipxe-script", "auto.ipxe", "The name of the iPXE script to use. used with remote-ipxe (http://192.168.2.3/<mac-addr>/auto.ipxe)")
	fs.StringVar(&c.CustomUserClass, "user-class", "iPXE", "A custom user-class (dhcp option 77) to use to determine when to pivot to serving the ipxe script from the ipxe-url flag.")
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
	if err := c.ValidateConfig(); err != nil {
		return err
	}

	return c.Run(ctx, args)
}

func (c *Config) Run(ctx context.Context, _ []string) error {
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
		proxy.WithTFTPAddr(ta),
		proxy.WithHTTPAddr(ha),
		proxy.WithIPXEAddr(ia),
	}
	if c.IPXEScript == "" {
		opts = append(opts, proxy.WithIPXEScript(c.IPXEScript))
	}
	if c.CustomUserClass != "" {
		opts = append(opts, proxy.WithUserClass(c.CustomUserClass))
	}
	h := proxy.NewHandler(ctx, opts...)

	rs, err := h.ServeRedirection(ctx, c.ProxyAddr)
	if err != nil {
		return err
	}

	bs, err := h.ServeBoot(ctx, c.ProxyAddr)
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
		rs.Close()
		bs.Close()
		return nil
	}
}
