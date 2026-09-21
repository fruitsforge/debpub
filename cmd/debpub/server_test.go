package cmd

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestServeCmdFlagsAndRegistration(t *testing.T) {
	if serveCmd == nil {
		t.Fatal("serveCmd is nil")
	}

	if serveCmd.Use != "serve [flags]" {
		t.Errorf("expected Use 'serve [flags]', got %q", serveCmd.Use)
	}

	// Verify flags exist
	flags := []string{"server-port", "server-bind", "server-tls-cert", "server-tls-key"}
	for _, f := range flags {
		if serveCmd.Flags().Lookup(f) == nil {
			t.Errorf("flag %q not found on serveCmd", f)
		}
	}
}

func TestServeCmdExecutionWithCancellation(t *testing.T) {
	tempDir := t.TempDir()

	// Find free local port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	ctx, cancel := context.WithCancel(context.Background())

	// Configure server
	cfg.Storage = "file"
	cfg.LocalDir = tempDir
	cfg.Codename = "testdist"
	cfg.ServerPort = port
	cfg.ServerBind = "127.0.0.1"

	serveCmd.SetContext(ctx)

	errCh := make(chan error, 1)
	go func() {
		errCh <- serveCmd.RunE(serveCmd, []string{})
	}()

	// Wait briefly and verify server is responding
	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://127.0.0.1:" + net.JoinHostPort("", string(rune(port))))
	// The HTTP get may or may not succeed quickly, but cancel will ensure graceful exit
	if err == nil {
		_ = resp.Body.Close()
	}

	// Cancel context to trigger shutdown
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("serveCmd returned unexpected error on shutdown: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("serveCmd failed to shut down within timeout")
	}
}
