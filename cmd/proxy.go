package cmd

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/go-playground/validator/v10"
	"github.com/jacobweinstock/proxydhcp/proxy"
	"github.com/peterbourgon/ff/v3"
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.universe.tf/netboot/dhcp4"
)

const appName = "proxydhcp"

type config struct {
	LogLevel        string `vname:"-loglevel" validate:"oneof=debug info"`
	TftpAddr        string `vname:"-tftp-addr" validate:"required,url"`
	HttpAddr        string `vname:"-http-addr" validate:"required,url"`
	IPXEURL         string `vname:"-ipxe-url" validate:"required,url"`
	Addr            string `vname:"-addr" validate:"hostname_port"`
	CustomUserClass string
	Log             logr.Logger
}

func Execute(ctx context.Context) error {

	rootCmd, rootConfig := New()

	if err := rootCmd.Parse(os.Args[1:]); err != nil {
		return err
	}

	rootConfig.Log = defaultLogger(rootConfig.LogLevel)

	return rootCmd.Run(ctx)
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

// newListener is a place holder for proxydhcp being a proper subcommand
// its goal is to serve proxydhcp requests.
func newListener(addr string) (*dhcp4.Conn, error) {
	conn, err := dhcp4.NewConn(formatAddr(addr))
	if err != nil {
		conn, err = dhcp4.NewSnooperConn(formatAddr(addr))
		if err != nil {
			return nil, err
		}
	}

	return conn, nil
}

// formatAddr will add 0.0.0.0 to a host:port combo that is without a host i.e. ":67".
func formatAddr(s string) string {
	if strings.HasPrefix(s, ":") {
		return "0.0.0.0" + s
	}
	return s
}

func New() (*ffcli.Command, *config) {
	var cfg config

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

func (c *config) RegisterFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.LogLevel, "loglevel", "info", "log level (optional)")
	fs.StringVar(&c.Addr, "addr", "0.0.0.0:67", "IP and port to listen on for proxydhcp requests.")
	fs.StringVar(&c.TftpAddr, "tftp-addr", "", "IP and URI of the TFTP server providing iPXE binaries (192.168.2.5/binaries).")
	fs.StringVar(&c.HttpAddr, "http-addr", "", "IP, port, and URI of the HTTP server providing iPXE binaries (i.e. 192.168.2.4:8080/binaries).")
	fs.StringVar(&c.IPXEURL, "ipxe-url", "", "A full url to an iPXE script (i.e. http://192.168.2.3/auto.ipxe).")
	fs.StringVar(&c.CustomUserClass, "user-class", "", "A custom user-class (dhcp option 77) to use to determine when to pivot to serving the ipxe script from the ipxe-url flag.")
}

func (c *config) validateConfig() error {
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
func (c *config) Exec(ctx context.Context, _ []string) error {
	if err := c.validateConfig(); err != nil {

		return err
	}

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
	tftp, _ := url.Parse(c.TftpAddr)
	ta := tftp.Host + tftp.Path
	htp, _ := url.Parse(c.HttpAddr)
	ha := htp.Host + htp.Path
	go proxy.ServeBoot(ctx, log, bootListener, ta, ha, c.IPXEURL, c.CustomUserClass)

	log.V(0).Info("starting proxydhcp", "addr1", c.Addr, "addr2", "0.0.0.0:4011")
	// proxy.Serve will block until the context (ctx) is canceled .
	proxy.Serve(ctx, log, redirectionListener, ta, ha, c.IPXEURL, c.CustomUserClass)
	return nil
}
