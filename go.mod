module github.com/jacobweinstock/proxydhcp

go 1.16

require (
	github.com/go-logr/logr v1.2.0
	github.com/go-logr/zapr v1.2.0
	github.com/go-playground/validator/v10 v10.9.0
	github.com/google/go-cmp v0.5.6
	github.com/hashicorp/go-multierror v1.1.1
	github.com/insomniacslk/dhcp v0.0.0-20211026125128-ad197bcd36fd
	github.com/olekukonko/tablewriter v0.0.5
	github.com/peterbourgon/ff/v3 v3.1.2
	github.com/pkg/errors v0.9.1
	go.uber.org/zap v1.19.0
	go.universe.tf/netboot v0.0.0-20210617221821-fc2840fa7b05
	golang.org/x/net v0.0.0-20210825183410-e898025ed96a
	inet.af/netaddr v0.0.0-20210903134321-85fa6c94624e
)

replace github.com/inetaf/netaddr v0.0.0-20210526175434-db50905a50be => inet.af/netaddr v0.0.0-20210526175434-db50905a50be
