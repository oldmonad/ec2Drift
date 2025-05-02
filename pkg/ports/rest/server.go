package rest

import (
	"context"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/oldmonad/ec2Drift/internal/app"
	"github.com/oldmonad/ec2Drift/pkg/errors"
	"github.com/oldmonad/ec2Drift/pkg/logger"
	"github.com/oldmonad/ec2Drift/pkg/ports/rest/handlers"
	"github.com/oldmonad/ec2Drift/pkg/utils/validator"
	"go.uber.org/zap"
)

// Server defines the behavior for starting, stopping, and retrieving the address of an HTTP server.
type Server interface {
	Start(port string) error
	Stop() error
	Address() string
}

// HttpServer implements the Server interface and manages HTTP lifecycle and handlers.
type HttpServer struct {
	// This struct can also be extended to handle different
	// kinds of handlers, not just this drift handler, and can act
	// as a hub for HTTP server primitives, e.g. (*http.Server)
	driftHandler *handlers.DriftHandler
	server       *http.Server
	stopCancel   context.CancelFunc
}

// NewServer creates a new instance of HttpServer with initialized drift handler.
func NewServer(app app.AppRunner, validator validator.Validator) Server {
	return &HttpServer{driftHandler: handlers.NewDriftHandler(app, validator)}
}

// Start starts the HTTP server on the specified port,
// initializes signal handling for graceful shutdown, and listens for requests.
func (s *HttpServer) Start(port string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/drift", s.driftHandler.HandleDrift)

	s.server = &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// Set up context that listens for interrupt/termination signals.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	s.stopCancel = stop
	defer stop()

	logger.Log.Info("Starting HTTP server", zap.String("addr", s.server.Addr))

	errChan := make(chan error, 1)

	// Start the server asynchronously and capture any unexpected errors.
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- errors.NewErrServerListen(s.server.Addr, err)
		}
	}()

	// Block until either an error occurs or a shutdown signal is received.
	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		logger.Log.Info("Received shutdown signal, stopping server")
		return s.Stop()
	}
}

// Stop performs a graceful shutdown of the server,
// allowing active requests up to 5 seconds to complete.
func (s *HttpServer) Stop() error {
	logger.Log.Info("Stopping HTTP server")
	if s.stopCancel != nil {
		s.stopCancel()
	}

	if s.server == nil {
		return nil
	}

	// Timeout context to ensure server shuts down gracefully within time window.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.server.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error("Server shutdown failed", zap.Error(err))
		return errors.NewErrServerShutdown(err)
	}

	logger.Log.Info("Server stopped successfully")
	return nil
}

// Address returns the bind address of the HTTP server.
func (s *HttpServer) Address() string {
	if s.server != nil {
		return s.server.Addr
	}
	return ""
}
