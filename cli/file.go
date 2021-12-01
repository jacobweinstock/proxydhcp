package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"reflect"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/imdario/mergo"
	"github.com/jacobweinstock/proxydhcp/authz/file"
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/pkg/errors"
	"github.com/tinkerbell/tink/protos/hardware"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const fileCLI = "file"

// FileCfg is the configuration for the file backend.
type FileCfg struct {
	Filename string
	Config
}

// File returns the cli command for the file backend.
func File() *ffcli.Command {
	cfg := &FileCfg{}
	fs := flag.NewFlagSet(fileCLI, flag.ExitOnError)
	RegisterFlagsFile(cfg, fs)

	return &ffcli.Command{
		Name:       fileCLI,
		ShortUsage: fileCLI,
		FlagSet:    fs,
		Exec: func(ctx context.Context, _ []string) error {
			return cfg.Exec(ctx, nil)
		},
	}
}

// RegisterFlagsFile registers the flags for the file backend.
func RegisterFlagsFile(cfg *FileCfg, fs *flag.FlagSet) {
	RegisterFlags(&cfg.Config, fs)
	fs.StringVar(&cfg.Filename, "filename", "", "filename to read data (required)")
}

type logger logr.Logger

// Transformer handles checking if the logger is empty or not.
func (l logger) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ == reflect.TypeOf(logr.Logger{}) {
		return func(dst, src reflect.Value) error {
			if dst.CanSet() {
				isZero := dst.MethodByName("GetSink")
				result := isZero.Call([]reflect.Value{})
				if result[0].IsNil() {
					dst.Set(src)
				}
			}
			return nil
		}
	}
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

// Exec serves proxy dhcp requests with the file backend.
func (f *FileCfg) Exec(ctx context.Context, _ []string) error {
	defaults := FileCfg{
		Config: Config{
			LogLevel:   "info",
			IPXEScript: "auto.ipxe",
			Log:        defaultLogger("info"),
		},
	}
	err := mergo.Merge(f, defaults, mergo.WithTransformers(logger{}))
	if err != nil {
		return err
	}

	f.Log = f.Log.WithName("proxydhcp")

	f.Log.Info("starting proxydhcp")

	saData, err := ioutil.ReadFile(f.Filename)
	if err != nil {
		return errors.Wrapf(err, "could not read file %q", f.Filename)
	}
	dsDB := []*hardware.Hardware{}
	if err := json.Unmarshal(saData, &dsDB); err != nil {
		return errors.Wrapf(err, "unable to parse configuration file %q", f.Filename)
	}

	fb := &file.File{DB: dsDB}
	f.Config.Authz = fb
	return f.Config.run(ctx, nil)
}
