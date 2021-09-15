FROM golang:1.16 as builder

WORKDIR /code
COPY go.mod go.sum /code/
RUN go mod download

COPY . /code
RUN CGO_ENABLED=0 go build -o proxydhcp

FROM scratch

COPY --from=builder /code/proxydhcp /proxydhcp

EXPOSE 67
ENTRYPOINT ["/proxydhcp"]