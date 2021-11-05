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

	"github.com/jacobweinstock/proxydhcp/proxy"
	"github.com/olekukonko/tablewriter"
	"github.com/peterbourgon/ff/v3"
	"github.com/peterbourgon/ff/v3/ffcli"
)

type Bin struct {
	ffcli.Command
	jsonOut bool
}

// Option for setting optional Client values.
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

func SupportedBins(_ context.Context, opts ...Option) *ffcli.Command {
	name := "binary"
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	defaultCfg := &Bin{
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

func (b *Bin) RegisterFlags(fs *flag.FlagSet) {
	fs.BoolVar(&b.jsonOut, "json", false, "output in json format")
}

// Execute function for this command.
func (b *Bin) Execute(_ context.Context, _ []string) error {
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
	for arch, ipxe := range proxy.DefaultsHTTP {
		output = append(output, spec{
			ID:     int(arch),
			Arch:   arch.String(),
			Binary: fmt.Sprintf(ipxe, "<IP>"),
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
		ipxe := proxy.Defaults[proxy.Architecture(elem)]
		table.Append([]string{strconv.Itoa(elem), proxy.Architecture(elem).String(), ipxe})
	}

	var unsortedDefaultsHTTP []int
	for arch := range proxy.DefaultsHTTP {
		unsortedDefaultsHTTP = append(unsortedDefaultsHTTP, int(arch))
	}
	sort.Ints(unsortedDefaultsHTTP)
	for _, elem := range unsortedDefaultsHTTP {
		ipxe := proxy.DefaultsHTTP[proxy.Architecture(elem)]
		table.Append([]string{strconv.Itoa(elem), proxy.Architecture(elem).String(), fmt.Sprintf(ipxe, "<IP>")})
	}

	table.Render()
}
