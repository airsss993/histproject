package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

type ServerConfig struct {
	Address           string
	ShutdownTimeout   time.Duration
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

type Server struct {
	httpServer      *http.Server
	shutdownTimeout time.Duration
	logger          *slog.Logger
}

func NewServer(cfg ServerConfig, handler http.Handler, logger *slog.Logger) *Server {
	applyServerDefaults(&cfg)
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{
		httpServer: &http.Server{
			Addr:              cfg.Address,
			Handler:           handler,
			ReadTimeout:       cfg.ReadTimeout,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
		},
		shutdownTimeout: cfg.ShutdownTimeout,
		logger:          logger,
	}
}

func (s *Server) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.httpServer.Addr, err)
	}

	serveErrors := make(chan error, 1)
	go func() {
		serveErrors <- s.httpServer.Serve(listener)
	}()

	s.logger.Info("HTTP server started", "address", listener.Addr().String())
	select {
	case err := <-serveErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer cancel()
	shutdownErr := s.httpServer.Shutdown(shutdownCtx)
	if shutdownErr != nil {
		_ = s.httpServer.Close()
	}

	serveErr := <-serveErrors
	if errors.Is(serveErr, http.ErrServerClosed) {
		serveErr = nil
	}
	if shutdownErr != nil || serveErr != nil {
		return errors.Join(shutdownErr, serveErr)
	}

	s.logger.Info("HTTP server stopped")
	return nil
}

func applyServerDefaults(cfg *ServerConfig) {
	if cfg.Address == "" {
		cfg.Address = ":7666"
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = 10 * time.Second
	}
	if cfg.ReadTimeout <= 0 {
		cfg.ReadTimeout = 15 * time.Second
	}
	if cfg.ReadHeaderTimeout <= 0 {
		cfg.ReadHeaderTimeout = 5 * time.Second
	}
	if cfg.WriteTimeout <= 0 {
		cfg.WriteTimeout = 30 * time.Second
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = 60 * time.Second
	}
}
