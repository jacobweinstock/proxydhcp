package proxy

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"reflect"

	"github.com/go-logr/logr"
	"github.com/go-playground/validator/v10"
	"github.com/hashicorp/go-multierror"
	"inet.af/netaddr"
)

// Allower is an interface for determining if a mac address is allowed to PXE boot or not.
type Allower interface {
	// Allow returns true if the mac address is allowed to PXE boot.
	Allow(ctx context.Context, mac net.HardwareAddr) bool
}

// Handler holds the data necessary to respond correctly to PXE enabled DHCP requests.
// It also holds context and a logger.
type Handler struct {
	Ctx context.Context `validate:"required"`
	Log logr.Logger     `validate:"required"`
	// TFTPAddr is the address to use for PXE clients requesting a tftp boot.
	TFTPAddr netaddr.IPPort `validate:"required"`
	// HTTPAddr is the address to use for PXE clients requesting a http boot.
	HTTPAddr netaddr.IPPort `validate:"required"`
	// IPXEAddr is the address to use for PXE clients requesting to boot from an iPXE script.
	IPXEAddr *url.URL `validate:"required"`
	// IPXEScript is the iPXE script to use for PXE clients requesting to boot from an iPXE script.
	IPXEScript string `validate:"required"`
	// UserClass is the custom user class (dhcp opt 77) to check if we are in a known iPXE binary.
	// When found, this allow us to stop serving iPXE binaries for PXE client requests and serve an iPXE script.
	UserClass string `validate:""`
	Allower   Allower
}

// Option for setting Handler values.
type Option func(*Handler)

// WithLogger sets the logger for the Handler struct.
func WithLogger(l logr.Logger) Option {
	return func(h *Handler) { h.Log = l }
}

// WithIPXEScript sets the IPXE script for the Handler struct.
func WithIPXEScript(s string) Option {
	return func(h *Handler) { h.IPXEScript = s }
}

// WithUserClass sets the user class for the Handler struct.
func WithUserClass(s string) Option {
	return func(h *Handler) { h.UserClass = s }
}

// WithAllower sets the Allower implementation.
func WithAllower(a Allower) Option {
	return func(h *Handler) { h.Allower = a }
}

// AllowAll is a default implementation of the Allower interface that will always return true for the Allow method.
type AllowAll struct{}

// Allow returns true for all mac addresses.
func (a AllowAll) Allow(_ context.Context, _ net.HardwareAddr) bool {
	return true
}

// NewHandler creates a new Handler struct. A few defaults are set here, but can be overridden by passing in options.
func NewHandler(ctx context.Context, tAddr, hAddr netaddr.IPPort, ipxeAddr *url.URL, opts ...Option) *Handler {
	defaultHandler := &Handler{
		Ctx:        ctx,
		Log:        logr.Discard(),
		TFTPAddr:   tAddr,
		HTTPAddr:   hAddr,
		IPXEAddr:   ipxeAddr,
		IPXEScript: "auto.ipxe",
		Allower:    AllowAll{},
	}
	for _, opt := range opts {
		opt(defaultHandler)
	}
	return defaultHandler
}

func validateHandler(h *Handler) error {
	v := validator.New()
	v.RegisterCustomTypeFunc(validateIPPORT, netaddr.IPPort{})
	v.RegisterCustomTypeFunc(validateURL, url.URL{})
	v.RegisterCustomTypeFunc(validateLogr, logr.Logger{})
	if err := v.Struct(h); err != nil {
		return multierror.Append(err, ErrInvalidHandler)
	}
	return nil
}

func validateIPPORT(field reflect.Value) interface{} {
	switch v := field.Interface().(type) {
	case netaddr.IPPort:
		if v.IsValid() {
			return fmt.Errorf("why does this work but returning v doesn't?")
		}
		return nil
	default:
		return nil
	}
}

func validateURL(field reflect.Value) interface{} {
	switch v := field.Interface().(type) {
	case url.URL:
		// TODO(jacobweinstock): validate host and port explicitly
		if _, err := url.Parse(v.String()); err == nil {
			return true
		}
		return nil
	default:
		return nil
	}
}

func validateLogr(field reflect.Value) interface{} {
	switch v := field.Interface().(type) {
	case logr.Logger:
		if v.GetSink() != nil {
			return true
		}
		return nil
	default:
		return nil
	}
}
