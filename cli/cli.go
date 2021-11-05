package cli

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/url"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	"github.com/go-playground/validator/v10"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
	"github.com/jacobweinstock/proxydhcp/proxy"
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/pkg/errors"
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
	go proxy.ServeBoot(ctx, c.Log, bootListener, "tftp://"+c.TFTPAddr, "http://"+c.HTTPAddr, c.IPXEAddr, c.IPXEScript, c.CustomUserClass)

	c.Log.Info("starting proxydhcp", "addr1", c.ProxyAddr, "addr2", "0.0.0.0:4011")
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
	h := &proxy.Handler{
		Ctx:        ctx,
		Log:        c.Log,
		TFTPAddr:   ta,
		HTTPAddr:   ha,
		IPXEAddr:   ia,
		IPXEScript: c.IPXEScript,
		UserClass:  c.CustomUserClass,
	}

	// for broadcast traffic we need to listen on all IPs
	laddr := net.UDPAddr{
		IP:   net.ParseIP("0.0.0.0"),
		Port: 67,
	}

	// server4.NewServer() will isolate listening to a specific interface.
	server, err := server4.NewServer(getInterfaceByIP(c.ProxyAddr), &laddr, h.Handler)
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
		server.Close()
		return nil
	}
}

func getInterfaceByIP(ip string) string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ipnet.IP.String() == ip {
					return iface.Name
				}
			}
		}
	}
	return ""
}
