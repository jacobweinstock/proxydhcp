package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/jacobweinstock/proxydhcp/proxy"
	"github.com/olekukonko/tablewriter"
	"github.com/peterbourgon/ff/v3"
	"github.com/peterbourgon/ff/v3/ffcli"
)

type Bin struct {
	ffcli.Command
	jsonOut bool
}

// Option for setting optional Client values
type Option func(*Bin)

func WithName(name string) Option {
	return func(cfg *Bin) {
		cfg.Name = name
	}
}

func WithShortUsage(shortUsage string) Option {
	return func(cfg *Bin) {
		cfg.ShortUsage = shortUsage
	}
}

func WithUsageFunc(usageFunc func(*ffcli.Command) string) Option {
	return func(cfg *Bin) {
		cfg.UsageFunc = usageFunc
	}
}

func WithFlagSet(flagSet *flag.FlagSet) Option {
	return func(cfg *Bin) {
		cfg.FlagSet = flagSet
	}
}

func WithOptions(opts ...ff.Option) Option {
	return func(cfg *Bin) {
		cfg.Options = append(cfg.Options, opts...)
	}
}

func SupportedBins(ctx context.Context, opts ...Option) *ffcli.Command {
	name := "binary"
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	defaultCfg := &Bin{
		Command: ffcli.Command{
			Name:       name,
			ShortUsage: fmt.Sprintf("%v returns the mapping of architecture to ipxe binary name", name),
			FlagSet:    fs,
		},
	}
	defaultCfg.Exec = defaultCfg.Execute
	defaultCfg.RegisterFlags(fs)

	for _, opt := range opts {
		opt(defaultCfg)
	}

	return &ffcli.Command{
		Name:        defaultCfg.Name,
		ShortUsage:  defaultCfg.ShortUsage,
		ShortHelp:   defaultCfg.ShortHelp,
		LongHelp:    defaultCfg.LongHelp,
		FlagSet:     defaultCfg.FlagSet,
		Options:     defaultCfg.Options,
		Subcommands: defaultCfg.Subcommands,
		Exec:        defaultCfg.Exec,
	}
}

func (b *Bin) RegisterFlags(fs *flag.FlagSet) {
	fs.BoolVar(&b.jsonOut, "json", false, "output in json format")
}

// Execute function for this command.
func (b *Bin) Execute(ctx context.Context, _ []string) error {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"ID", "Arch", "Binary"})

	for arch, ipxe := range proxy.Defaults {
		table.Append([]string{strconv.Itoa(int(arch)), arch.String(), ipxe})
	}
	for arch, ipxe := range proxy.DefaultsHTTP {
		table.Append([]string{strconv.Itoa(int(arch)), arch.String(), fmt.Sprintf(ipxe, "<your-ip>")})
	}
	table.Render()
	fmt.Println(b.jsonOut)

	return nil
}
