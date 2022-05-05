package proxydhcp

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/tonglil/buflogr"
)

func TestNoop_Handle(t *testing.T) {
	var buf bytes.Buffer
	var log logr.Logger = buflogr.NewWithBuffer(&buf)
	n := &Noop{
		Log: log,
	}
	n.Handle(nil, nil, nil)
	want := "INFO no handler specified. please specify a handler\n"
	if diff := cmp.Diff(buf.String(), want); diff != "" {
		t.Fatalf(diff)
	}
}

func TestNoop_HandleSTDOUT(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	n := &Noop{}
	n.Handle(nil, nil, nil)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)

	want := `noop.go:20: "level"=0 "msg"="no handler specified. please specify a handler"` + "\n"
	if diff := cmp.Diff(buf.String(), want); diff != "" {
		t.Fatalf(diff)
	}
}
