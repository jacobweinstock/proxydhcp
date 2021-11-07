module github.com/jacobweinstock/proxydhcp

go 1.17

require (
	github.com/go-logr/logr v1.2.0
	github.com/go-logr/zapr v1.2.0
	github.com/go-playground/validator/v10 v10.9.0
	github.com/google/go-cmp v0.5.2
	github.com/hashicorp/go-multierror v1.1.1
	github.com/insomniacslk/dhcp v0.0.0-20211026125128-ad197bcd36fd
	github.com/olekukonko/tablewriter v0.0.5
	github.com/peterbourgon/ff/v3 v3.1.2
	go.uber.org/zap v1.19.0
	golang.org/x/sync v0.0.0-20201020160332-67f06af15bc9
	inet.af/netaddr v0.0.0-20210903134321-85fa6c94624e
)

require (
	github.com/go-playground/locales v0.14.0 // indirect
	github.com/go-playground/universal-translator v0.18.0 // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/leodido/go-urn v1.2.1 // indirect
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/u-root/uio v0.0.0-20210528114334-82958018845c // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	go4.org/intern v0.0.0-20210108033219-3eb7198706b2 // indirect
	go4.org/unsafe/assume-no-moving-gc v0.0.0-20201222180813-1025295fd063 // indirect
	golang.org/x/crypto v0.0.0-20210711020723-a769d52b0f97 // indirect
	golang.org/x/net v0.0.0-20210825183410-e898025ed96a // indirect
	golang.org/x/sys v0.0.0-20210806184541-e5e7981a1069 // indirect
	golang.org/x/text v0.3.6 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
)

replace github.com/inetaf/netaddr v0.0.0-20210526175434-db50905a50be => inet.af/netaddr v0.0.0-20210526175434-db50905a50be
