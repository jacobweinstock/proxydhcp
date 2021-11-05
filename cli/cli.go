package cli

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	"github.com/go-playground/validator/v10"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
	"github.com/jacobweinstock/proxydhcp/proxy"
	reuseport "github.com/kavu/go_reuseport"
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/pkg/errors"
)

const appName = "proxy"

type Config struct {
	LogLevel        string `vname:"-loglevel" validate:"oneof=debug info"`
	TFTPAddr        string `vname:"-remote-tftp" validate:"required,url"`
	HTTPAddr        string `vname:"-remote-http" validate:"required,url"`
	IPXEAddr        string `vname:"-remote-ipxe" validate:"required"`
	IPXEScript      string `vname:"-ipxe-script"`
	ProxyAddr       string `vname:"-proxy-addr" validate:"hostname_port"`
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
		c.ProxyAddr = addr
	}
}

func WithIPXEURL(addr string) Opt {
	return func(c *Config) {
		c.IPXEAddr = addr
	}
}

func WithIPXEScriptName(name string) Opt {
	return func(c *Config) {
		c.IPXEScript = name
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
		IPXEAddr:        "",
		IPXEScript:      "auto.ipxe",
		ProxyAddr:       "0.0.0.0:67",
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
	/*
		redirectionListener, err := proxy.NewListener(c.ProxyAddr)
		if err != nil {
			return err
		}
		defer redirectionListener.Close()

		go func() {
			<-ctx.Done()
			redirectionListener.Close()
			c.Log.Info("shutting down proxydhcp", "addr", c.ProxyAddr)
		}()
	*/

	bootListener, err := net.ListenPacket("udp4", fmt.Sprintf("%s:%d", "0.0.0.0", 4011))
	if err != nil {
		return err
	}
	defer bootListener.Close()
	go func() {
		<-ctx.Done()
		bootListener.Close()
		c.Log.Info("shutting down proxydhcp", "addr", c.ProxyAddr)
	}()
	go proxy.ServeBoot(ctx, c.Log, bootListener, c.TFTPAddr, c.HTTPAddr, c.IPXEAddr, c.IPXEScript, c.CustomUserClass)

	c.Log.Info("starting proxydhcp", "addr1", c.ProxyAddr, "addr2", "0.0.0.0:4011")
	// proxy.Serve will block until the context (ctx) is canceled .
	//proxy.Serve(ctx, c.Log, redirectionListener, c.TFTPAddr, c.HTTPAddr, c.IPXEAddr, c.IPXEScript, c.CustomUserClass)
	h := &proxy.Handler{
		Ctx:        ctx,
		Log:        c.Log,
		TFTPAddr:   c.TFTPAddr,
		HTTPAddr:   c.HTTPAddr,
		IPXEAddr:   c.IPXEAddr,
		IPXEScript: c.IPXEScript,
		UserClass:  c.CustomUserClass,
	}
	listener, err := reuseport.ListenPacket("udp4", c.ProxyAddr)
	if err != nil {
		return err
	}
	defer listener.Close()
	laddr := net.UDPAddr{
		IP:   net.ParseIP("0.0.0.0"),
		Port: 67,
	}
	// server4.NewServer(ifname string is ok to be "" because we are passing in our own conn
	server, err := server4.NewServer("", &laddr, h.Handler, server4.WithConn(listener))
	if err != nil {
		log.Fatal(err)
	}
	errCh := make(chan error)
	go func() {
		errCh <- server.Serve()
	}()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		listener.Close()
		return nil
	}
}
