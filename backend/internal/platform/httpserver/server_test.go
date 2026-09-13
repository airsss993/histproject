package httpserver

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"
)

func TestNewServerAppliesTimeouts(t *testing.T) {
	cfg := ServerConfig{
		Address:           "127.0.0.1:0",
		ShutdownTimeout:   3 * time.Second,
		ReadTimeout:       4 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      6 * time.Second,
		IdleTimeout:       7 * time.Second,
	}

	server := NewServer(cfg, http.NotFoundHandler(), discardLogger())

	if server.httpServer.Addr != cfg.Address {
		t.Errorf("Addr = %q, want %q", server.httpServer.Addr, cfg.Address)
	}
	if server.httpServer.ReadTimeout != cfg.ReadTimeout {
		t.Errorf("ReadTimeout = %s, want %s", server.httpServer.ReadTimeout, cfg.ReadTimeout)
	}
	if server.httpServer.ReadHeaderTimeout != cfg.ReadHeaderTimeout {
		t.Errorf("ReadHeaderTimeout = %s, want %s", server.httpServer.ReadHeaderTimeout, cfg.ReadHeaderTimeout)
	}
	if server.httpServer.WriteTimeout != cfg.WriteTimeout {
		t.Errorf("WriteTimeout = %s, want %s", server.httpServer.WriteTimeout, cfg.WriteTimeout)
	}
	if server.httpServer.IdleTimeout != cfg.IdleTimeout {
		t.Errorf("IdleTimeout = %s, want %s", server.httpServer.IdleTimeout, cfg.IdleTimeout)
	}
	if server.shutdownTimeout != cfg.ShutdownTimeout {
		t.Errorf("shutdownTimeout = %s, want %s", server.shutdownTimeout, cfg.ShutdownTimeout)
	}
}

func TestServerRunReturnsListenError(t *testing.T) {
	server := NewServer(ServerConfig{Address: "invalid address"}, http.NotFoundHandler(), discardLogger())

	err := server.Run(context.Background())

	if err == nil {
		t.Fatal("Run() error = nil")
	}
}

func TestServerRunStopsAfterContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	server := NewServer(ServerConfig{
		Address:         "127.0.0.1:0",
		ShutdownTimeout: time.Second,
	}, http.NotFoundHandler(), discardLogger())

	if err := server.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
