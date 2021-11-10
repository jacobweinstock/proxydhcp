package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"

	"github.com/insomniacslk/dhcp/iana"
	"github.com/jacobweinstock/proxydhcp/proxy"
	"github.com/olekukonko/tablewriter"
	"github.com/peterbourgon/ff/v3"
	"github.com/peterbourgon/ff/v3/ffcli"
)

type bin struct {
	ffcli.Command
	jsonOut bool
}

// Option for setting optional Client values.
type Option func(*bin)

// WithName sets the name of the command.
func WithName(name string) Option {
	return func(cfg *bin) {
		cfg.Name = name
	}
}

// WithShortUsage sets the short usage of the command.
func WithShortUsage(shortUsage string) Option {
	return func(cfg *bin) {
		cfg.ShortUsage = shortUsage
	}
}

// WithUsageFunc sets the usage function for the command.
func WithUsageFunc(usageFunc func(*ffcli.Command) string) Option {
	return func(cfg *bin) {
		cfg.UsageFunc = usageFunc
	}
}

// WithFlagSet adds a flag set to the command.
func WithFlagSet(flagSet *flag.FlagSet) Option {
	return func(cfg *bin) {
		cfg.FlagSet = flagSet
	}
}

// WithOptions adds command options to the command.
func WithOptions(opts ...ff.Option) Option {
	return func(cfg *bin) {
		cfg.Options = append(cfg.Options, opts...)
	}
}

// SupportedBins returns the command for printing the arch to iPXE binary mapping.
func SupportedBins(_ context.Context, opts ...Option) *ffcli.Command {
	name := "binary"
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	defaultCfg := &bin{
		Command: ffcli.Command{
			Name:       name,
			ShortUsage: fmt.Sprintf("%v returns the mapping of supported architecture to ipxe binary name", name),
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

// RegisterFlags registers the binary command flags.
func (b *bin) RegisterFlags(fs *flag.FlagSet) {
	fs.BoolVar(&b.jsonOut, "json", false, "output in json format")
}

// Execute function for this command.
func (b *bin) Execute(_ context.Context, _ []string) error {
	if b.jsonOut {
		jsonOut(os.Stdout)
	} else {
		table(os.Stdout)
	}

	return nil
}

func jsonOut(w io.Writer) {
	type spec struct {
		ID     int    `json:"id"`
		Arch   string `json:"arch"`
		Binary string `json:"binary"`
	}
	output := make([]spec, 0)
	for arch, ipxe := range proxy.ArchToBootFile {
		output = append(output, spec{
			ID:     int(arch),
			Arch:   arch.String(),
			Binary: ipxe,
		})
	}
	for arch, ipxe := range proxy.ArchToBootFile {
		output = append(output, spec{
			ID:     int(arch),
			Arch:   arch.String(),
			Binary: ipxe,
		})
	}
	out, err := json.Marshal(output)
	if err != nil {
		panic(err)
	}
	fmt.Fprintln(w, string(out))
}

func table(w io.Writer) {
	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"ID", "Arch", "Binary"})

	var unsortedDefaults []int
	for arch := range proxy.ArchToBootFile {
		unsortedDefaults = append(unsortedDefaults, int(arch))
	}
	sort.Ints(unsortedDefaults)
	for _, elem := range unsortedDefaults {
		ipxe := proxy.ArchToBootFile[iana.Arch(elem)]
		table.Append([]string{strconv.Itoa(elem), iana.Arch(elem).String(), ipxe})
	}

	table.Render()
}
