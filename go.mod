module github.com/jacobweinstock/proxydhcp

go 1.16

require (
	github.com/fsnotify/fsnotify v1.4.7
	github.com/go-logr/logr v1.1.0
	github.com/go-logr/zapr v1.1.0
	github.com/go-playground/universal-translator v0.18.0 // indirect
	github.com/go-playground/validator v9.31.0+incompatible
	github.com/goccy/go-yaml v1.9.3
	github.com/google/go-cmp v0.5.6
	github.com/leodido/go-urn v1.2.1 // indirect
	github.com/libp2p/go-reuseport v0.0.2
	github.com/peterbourgon/ff/v3 v3.1.0
	github.com/pkg/errors v0.9.1
	go.uber.org/zap v1.19.1
	go.universe.tf/netboot v0.0.0-20210617221821-fc2840fa7b05
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	gopkg.in/go-playground/assert.v1 v1.2.1 // indirect
	inet.af/netaddr v0.0.0-20210903134321-85fa6c94624e
)

replace github.com/inetaf/netaddr v0.0.0-20210526175434-db50905a50be => inet.af/netaddr v0.0.0-20210526175434-db50905a50be
