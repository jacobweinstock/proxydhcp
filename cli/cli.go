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
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/pkg/errors"
)

const appName = "proxy"

type Config struct {
	LogLevel        string `vname:"-loglevel" validate:"oneof=debug info"`
	TFTPAddr        string `vname:"-tftp-addr" validate:"required,url"`
	HTTPAddr        string `vname:"-http-addr" validate:"required,url"`
	IPXEURL         string `vname:"-ipxe-url" validate:"required"`
	Addr            string `vname:"-addr" validate:"hostname_port"`
	CustomUserClass string
	Log             logr.Logger
}

// Option for setting optional Client values.
type Opt func(*Config)

func WithLogger(l logr.Logger) Opt {
	return func(c *Config) {
		c.Log = l
	}
}

func WithLogLevel(l string) Opt {
	return func(c *Config) {
		c.LogLevel = l
	}
}

func WithCustomUserClass(class string) Opt {
	return func(c *Config) {
		c.CustomUserClass = class
	}
}

func WithAddr(addr string) Opt {
	return func(c *Config) {
		c.Addr = addr
	}
}

func WithIPXEURL(url string) Opt {
	return func(c *Config) {
		c.IPXEURL = url
	}
}

func WithHTTPAddr(addr string) Opt {
	return func(c *Config) {
		c.HTTPAddr = addr
	}
}

func WithTFTPAddr(addr string) Opt {
	return func(c *Config) {
		c.TFTPAddr = addr
	}
}

func NewConfig(opts ...Opt) *Config {
	c := &Config{
		LogLevel:        "info",
		TFTPAddr:        "0.0.0.0:69",
		HTTPAddr:        "0.0.0.0:8080",
		IPXEURL:         "",
		Addr:            "0.0.0.0:67",
		CustomUserClass: "iPXE",
		Log:             logr.Discard(),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func ProxyDHCP(_ context.Context) (*ffcli.Command, *Config) {
	fs := flag.NewFlagSet(appName, flag.ExitOnError)
	cfg := &Config{
		Log:  logr.Discard(),
		Addr: "0.0.0.0:67",
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
	fs.StringVar(&c.Addr, "addr", "0.0.0.0:67", "IP and port to listen on for proxydhcp requests.")
	fs.StringVar(&c.TFTPAddr, "remote-tftp", "", "IP and URI of the TFTP server providing iPXE binaries (192.168.2.5/binaries).")
	fs.StringVar(&c.HTTPAddr, "remote-http", "", "IP, port, and URI of the HTTP server providing iPXE binaries (i.e. 192.168.2.4:8080/binaries).")
	fs.StringVar(&c.IPXEURL, "remote-ipxe-script", "", "A full url to an iPXE script (i.e. http://192.168.2.3/%v/auto.ipxe).")
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
		// s := "'%v' is not a valid for flag %v\n"
		for _, msg := range err.(validator.ValidationErrors) {
			errMsg = append(errMsg, fmt.Sprintf("%v '%v' not valid: '%v'", msg.Field(), msg.Value(), msg.Tag()))
		}
		errMsg = append(errMsg, "\n")
		return errors.Wrap(flag.ErrHelp, strings.Join(errMsg, "\n"))
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
	if c.Log.GetSink() == nil {
		c.Log = logr.Discard()
	}
	c.Log = c.Log.WithName("proxydhcp")
	redirectionListener, err := proxy.NewListener(c.Addr)
	if err != nil {
		return err
	}
	defer redirectionListener.Close()
	log := c.Log

	go func() {
		<-ctx.Done()
		redirectionListener.Close()
		log.Info("shutting down proxydhcp", "addr", c.Addr)
	}()

	bootListener, err := net.ListenPacket("udp4", fmt.Sprintf("%s:%d", "0.0.0.0", 4011))
	if err != nil {
		return err
	}
	defer bootListener.Close()
	go func() {
		<-ctx.Done()
		bootListener.Close()
		log.Info("shutting down proxydhcp", "addr", c.Addr)
	}()
	go proxy.ServeBoot(ctx, log, bootListener, c.TFTPAddr, c.HTTPAddr, c.IPXEURL, c.CustomUserClass)

	log.Info("starting proxydhcp", "addr1", c.Addr, "addr2", "0.0.0.0:4011")
	// proxy.Serve will block until the context (ctx) is canceled .
	proxy.Serve(ctx, log, redirectionListener, c.TFTPAddr, c.HTTPAddr, c.IPXEURL, c.CustomUserClass)
	return nil
}
