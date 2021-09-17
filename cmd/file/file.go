package file

import (
	"context"
	"flag"

	"github.com/go-playground/validator"
	"github.com/jacobweinstock/proxydhcp/backend/file"
	"github.com/jacobweinstock/proxydhcp/cmd/root"
	"github.com/jacobweinstock/proxydhcp/proxy"
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/pkg/errors"
)

const (
	subCmd = "file"
)

type Config struct {
	rootConfig *root.Config
	FilePath   string `validate:"required,file"`
}

func New(rootConfig *root.Config) *ffcli.Command {
	cfg := Config{
		rootConfig: rootConfig,
	}

	fs := flag.NewFlagSet(subCmd, flag.ExitOnError)
	fs.StringVar(&cfg.FilePath, "path", "", "path to file with mac to <> mappings.")

	return &ffcli.Command{
		Name:       subCmd,
		ShortUsage: "proxydhcp file",
		ShortHelp:  "run the proxydhcp server using the file backend.",
		FlagSet:    fs,
		Exec:       cfg.Exec,
	}
}

// Exec function for this command.
func (c *Config) Exec(ctx context.Context, _ []string) error {
	if err := c.validateConfig(ctx); err != nil {
		return err
	}

	backend := &file.Config{FilePath: c.FilePath}
	if err := backend.FirstLoad(); err != nil {
		return err
	}

	go backend.Watcher()

	listener, err := proxy.NewListener(c.rootConfig.Addr)
	if err != nil {
		return err
	}
	defer listener.Close()
	log := c.rootConfig.Log.WithValues("backend", subCmd)

	go func() {
		<-ctx.Done()
		listener.Close()
		log.V(0).Info("shutting down proxydhcp", "addr", c.rootConfig.Addr)
	}()

	log.V(0).Info("starting proxydhcp", "addr", c.rootConfig.Addr)
	// proxy.Serve will block until the context (ctx) is canceled .
	proxy.Serve(ctx, log, listener, backend)

	return nil
}

func (c *Config) validateConfig(_ context.Context) error {
	if err := validator.New().Struct(c); err != nil {
		var errMsg []interface{}
		s := "'%v' is not a valid %v"
		for _, msg := range err.(validator.ValidationErrors) {
			errMsg = append(errMsg, msg.Value(), msg.Field())
		}

		return errors.Wrapf(flag.ErrHelp, s, errMsg...)
	}

	return nil
}
