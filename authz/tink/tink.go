// Package tink handles comminicating with the Tink server.
package tink

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/tinkerbell/tink/protos/hardware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

const (
	schemeFile  = "file"
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)

// Tinkerbell is the struct for communicating with Tink server.
type Tinkerbell struct {
	Client hardware.HardwareServiceClient
	Log    logr.Logger
}

// Allow handles communicating with Tink server to determine if a MAC address should be allowed to PXE boot or not.
func (t Tinkerbell) Allow(ctx context.Context, mac net.HardwareAddr) bool {
	hw, err := t.Client.ByMAC(ctx, &hardware.GetRequest{Mac: mac.String()})
	if err != nil {
		fmt.Println("==========")
		fmt.Printf("%T\n", err)
		errStatus, _ := status.FromError(err)
		fmt.Println(errStatus.Message())
		fmt.Println(errStatus.Code())
		fmt.Println("==========")
		err = errors.Wrap(err, errStatus.Code().String())
		t.Log.Error(err, "failed to get hardware info")
		return false
		// return false, fmt.Errorf("failed to get hardware info: %w", err)
	}
	for _, elem := range hw.GetNetwork().GetInterfaces() {
		found, err := net.ParseMAC(elem.GetDhcp().GetMac())
		if err != nil {
			continue
		}
		if found.String() == mac.String() {
			return elem.GetNetboot().GetAllowPxe()
		}
	}

	return false
}

// SetupClient is a small control loop to create a tink server client.
// it keeps trying so that if the problem is temporary or can be resolved and the
// this doesn't stop and need to be restarted by an outside process or person.
func SetupClient(ctx context.Context, log logr.Logger, tlsVal string, tink string) (*grpc.ClientConn, error) {
	if tink == "" {
		return nil, errors.New("tink server address is required")
	}
	// setup tink server grpc client
	dialOpt, err := grpcTLS(tlsVal)
	if err != nil {
		log.Error(err, "unable to create gRPC client TLS dial option")
		return nil, err
	}

	grpcClient, err := grpc.DialContext(ctx, tink, dialOpt)
	if err != nil {
		log.Error(err, "error connecting to tink server")
		return nil, err
	}

	return grpcClient, nil
}

// toCreds takes a byte string, assumed to be a tls cert, and creates a transport credential.
func toCreds(pemCerts []byte) credentials.TransportCredentials {
	cp := x509.NewCertPool()
	ok := cp.AppendCertsFromPEM(pemCerts)
	if !ok {
		return nil
	}
	return credentials.NewClientTLSFromCert(cp, "")
}

// loadTLSSecureOpts handles taking a string that is assumed to be a boolean
// and creating a grpc.DialOption for TLS.
// If the value is true, the server has a cert from a well known CA.
// If the value is false, the server is not using TLS

// loadTLSFromFile handles reading in a cert file and forming a TLS grpc.DialOption

// loadTLSFromHTTP handles reading a cert from an HTTP endpoint and forming a TLS grpc.DialOption

// grpcTLS is the logic for how/from where TLS should be loaded.
func grpcTLS(tlsVal string) (grpc.DialOption, error) {
	u, err := url.Parse(tlsVal)
	if err != nil {
		return nil, errors.Wrap(err, "must be file://, http://, or string boolean")
	}
	switch u.Scheme {
	case "":
		secure, err := strconv.ParseBool(tlsVal)
		if err != nil {
			return nil, errors.WithMessagef(err, "expected boolean, got: %v", tlsVal)
		}
		var dialOpt grpc.DialOption
		if secure {
			// 1. the server has a cert from a well known CA - grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig))
			dialOpt = grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12}))
		} else {
			// 2. the server is not using TLS - grpc.WithInsecure()
			dialOpt = grpc.WithInsecure()
		}
		return dialOpt, nil
	case schemeFile:
		// 3. the server has a self-signed cert and the cert have be provided via file/env/flag -
		data, err := os.ReadFile(filepath.Join(u.Host, u.Path))
		if err != nil {
			return nil, err
		}
		if len(data) == 0 {
			return nil, errors.New("no certificate found")
		}
		return grpc.WithTransportCredentials(toCreds(data)), nil
	case schemeHTTP, schemeHTTPS:
		// 4. the server has a self-signed cert and the cert needs to be grabbed from a URL -
		req, err := http.NewRequestWithContext(context.Background(), "GET", tlsVal, http.NoBody)
		if err != nil {
			return nil, err
		}
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		cert, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		if len(cert) == 0 {
			return nil, errors.New("no certificate found")
		}
		// TODO(jacobweinstock): []byte is a valid TLS cert.
		return grpc.WithTransportCredentials(toCreds(cert)), nil
	}
	return nil, fmt.Errorf("not an expected value: %v", tlsVal)
}
