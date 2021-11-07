package proxy

import (
	"context"
	"fmt"
	"net/url"
	"reflect"

	"github.com/go-logr/logr"
	"github.com/go-playground/validator/v10"
	"github.com/hashicorp/go-multierror"
	"inet.af/netaddr"
)

var ErrInvalid = fmt.Errorf("validation failed")

type Handler struct {
	Ctx        context.Context `validate:"required"`
	Log        logr.Logger     `validate:"required"`
	TFTPAddr   netaddr.IPPort  `validate:"required"`
	HTTPAddr   netaddr.IPPort  `validate:"required"`
	IPXEAddr   *url.URL        `validate:"required"`
	IPXEScript string          `validate:"required"`
	UserClass  string          `validate`
}

// Option for setting Handler values.
type Option func(*Handler)

func WithLogger(l logr.Logger) Option {
	return func(h *Handler) { h.Log = l }
}

func WithTFTPAddr(ta netaddr.IPPort) Option {
	return func(h *Handler) { h.TFTPAddr = ta }
}

func WithHTTPAddr(ha netaddr.IPPort) Option {
	return func(h *Handler) { h.HTTPAddr = ha }
}

func WithIPXEAddr(u *url.URL) Option {
	return func(h *Handler) { h.IPXEAddr = u }
}

func WithIPXEScript(s string) Option {
	return func(h *Handler) { h.IPXEScript = s }
}

func WithUserClass(s string) Option {
	return func(h *Handler) { h.UserClass = s }
}

func NewHandler(ctx context.Context, opts ...Option) *Handler {
	defaultHandler := &Handler{
		Ctx:        ctx,
		Log:        logr.Discard(),
		IPXEScript: "auto.ipxe",
	}
	for _, opt := range opts {
		opt(defaultHandler)
	}
	return defaultHandler
}

func validate(h *Handler) error {
	v := validator.New()
	v.RegisterCustomTypeFunc(validateIPPORT, netaddr.IPPort{})
	v.RegisterCustomTypeFunc(validateURL, url.URL{})
	v.RegisterCustomTypeFunc(validateLogr, logr.Logger{})
	if err := v.Struct(h); err != nil {
		return multierror.Append(err, ErrInvalid)
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