package main

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-logr/stdr"
	"github.com/gorilla/mux"
	"github.com/jacobweinstock/proxydhcp"
	"github.com/jacobweinstock/proxydhcp/proxy"
	"inet.af/netaddr"
)

func main() {
	// main2()

	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGHUP, syscall.SIGTERM)
	defer done()
	logger := stdr.New(log.New(os.Stdout, "", log.Lshortfile))

	tftp := netaddr.IPPortFrom(netaddr.IPv4(192, 168, 2, 160), 69)
	http := netaddr.IPPortFrom(netaddr.IPv4(192, 168, 2, 160), 8080)
	ipxeScript := &url.URL{Scheme: "http", Host: "192.168.2.160", Path: "/auto.ipxe"}
	proxyHandler := proxy.NewHandler(ctx, tftp, http, ipxeScript, proxy.WithLogger(logger))

	s := &proxydhcp.Server{
		Log:  logger,
		Addr: netaddr.IPPortFrom(netaddr.IPv4(0, 0, 0, 0), 67),
	}
	logger.Info("starting server")
	// proxyHandler := &proxydhcp.Noop{Log: logger}
	go func() {
		<-ctx.Done()
		logger.Error(s.Shutdown(), "shutdown")
	}()

	logger.Error(s.ListenAndServe(ctx, proxyHandler), "down")

	/*

		conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(0, 0, 0, 0), Port: 67})
		if err != nil {
			logger.Error(err, "failed to listen")
			return
		}
		go func() {
			logger.Error(proxydhcp.Serve(ctx, conn, nil), "failed to serve")
		}()
		<-ctx.Done()
		logger.Error(conn.Close(), "close")
		time.Sleep(time.Second * 5)
		logger.Info("done")
	*/
}

func TestEndpoint(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	w.Write([]byte("Test is what we usually do"))
}

func main2() {
	router := mux.NewRouter()
	router.HandleFunc("/test", TestEndpoint).Methods("GET")

	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGHUP, syscall.SIGTERM)
	defer done()

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
		log.Print("listen: done")
	}()
	log.Print("Server Started")

	<-ctx.Done()
	log.Print("Server Stopped")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		// extra handling here
		cancel()
	}()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server Shutdown Failed:%+v", err)
	}
	log.Print("Server Exited Properly")
	time.Sleep(time.Second * 5)
}
