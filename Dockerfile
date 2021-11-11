# docker build -t proxydhcp .
# docker run -it --rm proxydhcp -tftp-addr 192.168.2.109 -http-addr 192.168.2.109 -ipxe-url http://192.168.2.109/auto.ipxe
FROM golang:1.17 as builder

WORKDIR /code
COPY go.mod go.sum /code/
RUN go mod download

COPY . /code
RUN CGO_ENABLED=0 go build -o proxydhcp

FROM scratch

COPY --from=builder /code/proxydhcp /proxydhcp
EXPOSE 67/udp
EXPOSE 4011/udp

ENTRYPOINT ["/proxydhcp"]
