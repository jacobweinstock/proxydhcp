# proxydhcp

standalone proxydhcp server

```bash
go run main.go proxy -remote-tftp 192.168.2.225:69 -remote-http 192.168.2.225:80 -remote-ipxe http://192.168.2.225:8080 -proxy-addr 192.168.2.225

go run main.go proxy -remote-tftp 192.168.2.225:69 -remote-http 192.168.2.225:80 -remote-ipxe http://192.168.2.225:8080 -proxy-addr 192.168.1.34
```
