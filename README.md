[![Test and Build](https://github.com/jacobweinstock/proxydhcp/actions/workflows/ci.yaml/badge.svg)](https://github.com/jacobweinstock/proxydhcp/actions/workflows/ci.yaml)
[![codecov](https://codecov.io/gh/jacobweinstock/proxydhcp/branch/main/graph/badge.svg)](https://codecov.io/gh/jacobweinstock/proxydhcp)
[![Go Report Card](https://goreportcard.com/badge/github.com/jacobweinstock/proxydhcp)](https://goreportcard.com/report/github.com/jacobweinstock/proxydhcp)
[![Go Reference](https://pkg.go.dev/badge/github.com/jacobweinstock/proxydhcp.svg)](https://pkg.go.dev/github.com/jacobweinstock/proxydhcp)

# proxydhcp

`proxydhcp` is a standalone Proxy DHCP server.

> [A] Proxy DHCP server behaves much like a DHCP server by listening for ordinary DHCP client traffic and responding to certain client requests. However, unlike the DHCP server, the PXE Proxy DHCP server does not administer network addresses, and it only responds to clients that identify themselves as PXE clients.
> The responses given by the PXE Proxy DHCP server contain the mechanism by which the client locates the boot servers or the network addresses and descriptions of the supported, compatible boot servers."
> -- [IBM](https://www.ibm.com/docs/en/aix/7.1?topic=protocol-preboot-execution-environment-proxy-dhcp-daemon)

Currently, `proxydhcp` only supports booting to [iPXE](https://ipxe.org/) binaries and scripts. Run `proxydhcp binary` to see the supported architectures and iPXE binaries.

## Installation

```bash
# requires Go >= 1.17
go install github.com/jacobweinstock/proxydhcp@v0.4.4
# usage
proxydhcp proxy -h
```

```bash
# requires Go >= 1.17
make build
# usage
./bin/proxydhcp-<arch> proxy -h
```

```bash
docker pull ghcr.io/jacobweinstock/proxydhcp:0.4.4
# usage
docker run -it --rm -p 67:67/udp -p 4011:4011/udp ghcr.io/jacobweinstock/proxydhcp:0.4.4 proxy -h
```

```bash
docker build -t proxydhcp .
# usage
docker run -it --rm -p 67:67/udp -p 4011:4011/udp proxydhcp proxy -h
```

## Usage

```bash
❯ proxydhcp proxy -h # docker run -it --rm -p 67:67/udp -p 4011:4011/udp ghcr.io/jacobweinstock/proxydhcp:0.4.4 proxy -h
USAGE
  proxy runs the proxyDHCP server

FLAGS
  -loglevel info                 log level (optional)
  -proxy-addr 0.0.0.0            IP associated to the network interface to listen on for proxydhcp requests.
  -remote-http ...               IP, port, and URI of the HTTP server providing iPXE binaries (i.e. 192.168.2.4:80).
  -remote-ipxe ...               A url where an iPXE script is served (i.e. http://192.168.2.3:8080).
  -remote-ipxe-script auto.ipxe  The name of the iPXE script to use. used with remote-ipxe (http://192.168.2.3/<mac-addr>/auto.ipxe)
  -remote-tftp ...               IP and URI of the TFTP server providing iPXE binaries (192.168.2.5:69).
  -user-class ...                A custom user-class (dhcp option 77) to use to determine when to pivot to serving the ipxe script (-remote-ipxe-script flag).

```

```bash
❯ proxydhcp binary -h # docker run -it --rm ghcr.io/jacobweinstock/proxydhcp:0.4.4 binary -h
USAGE
  binary returns the mapping of supported architecture to ipxe binary name

FLAGS
  -json=false  output in json format

```
