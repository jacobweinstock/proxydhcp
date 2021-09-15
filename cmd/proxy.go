package cmd

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/jacobweinstock/proxyDHCP/app"
	"github.com/jacobweinstock/proxyDHCP/proxy"
	"github.com/peterbourgon/ff/v3"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.universe.tf/netboot/dhcp4"
)

// Execute sets up the config and logging, then runs the proxyDHCP Server.
func Execute(ctx context.Context) error {
	fs := flag.NewFlagSet("proxyDHCP", flag.ExitOnError)
	addr := fs.String("addr", "0.0.0.0:67", "IP and port to listen on for proxyDHCP requests.")
	ll := fs.String("loglevel", "info", "log level (optional)")
	err := ff.Parse(fs, os.Args[1:], ff.WithEnvVarPrefix("PROXYDHCP"))
	if err != nil {
		return err
	}

	log := defaultLogger(*ll)

	listener, err := newListener(*addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	go func() {
		<-ctx.Done()
		listener.Close()
		log.V(0).Info("shutting down proxyDHCP", "addr", *addr)
	}()

	log.V(0).Info("starting proxyDHCP", "addr", *addr)
	// proxy.Serve will block until the context (ctx) is canceled.
	proxy.Serve(ctx, log, listener, app.Default{URL: &url.URL{Host: "192.168.2.109"}})

	return nil
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

// newListener is a place holder for proxyDHCP being a proper subcommand
// its goal is to serve proxyDHCP requests.
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
