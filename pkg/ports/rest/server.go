package rest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/oldmonad/ec2Drift/internal/app"
	cerrors "github.com/oldmonad/ec2Drift/pkg/errors"
	"github.com/oldmonad/ec2Drift/pkg/logger"
	"github.com/oldmonad/ec2Drift/pkg/parser"
	"github.com/oldmonad/ec2Drift/pkg/ports"
	"go.uber.org/zap"
)

var (
	HandleDriftCheck = handleDriftCheck
)

type HTTPServer interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
}

type standardHTTPServer struct {
	*http.Server
}

func (s *standardHTTPServer) ListenAndServe() error {
	return s.Server.ListenAndServe()
}

func (s *standardHTTPServer) Shutdown(ctx context.Context) error {
	return s.Server.Shutdown(ctx)
}

var NewHTTPServer = func(addr string, handler http.Handler) HTTPServer {
	return &standardHTTPServer{
		Server: &http.Server{
			Addr:    addr,
			Handler: handler,
		},
	}
}

func StartServer(appInstance app.AppRunner, port string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/drift", func(w http.ResponseWriter, r *http.Request) {
		handleDriftCheck(appInstance, w, r)
	})

	server := NewHTTPServer(":"+port, mux)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Log.Info("Starting HTTP server", zap.String("addr", ":"+port))

	errChan := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("server error: %w", err)
		}
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		logger.Log.Info("Received shutdown signal, stopping server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Log.Error("Server shutdown failed", zap.Error(err))
			return fmt.Errorf("shutdown error: %w", err)
		}

		logger.Log.Info("Server stopped successfully")
		return nil
	}
}
func handleDriftCheck(appInstance app.AppRunner, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	var req struct {
		Attrs  []string `json:"attributes"`
		Format string   `json:"format"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}
	if req.Attrs == nil {
		req.Attrs = []string{}
	}
	format := parser.Terraform
	if req.Format == "json" {
		format = parser.JSON
	} else if req.Format != "" && req.Format != "terraform" {
		format = parser.Terraform
	}
	err := appInstance.Run(r.Context(), req.Attrs, format, ports.HTTP)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "drift detected"):
			sendResponse(w, http.StatusOK, map[string]interface{}{
				"drift_detected": true,
				"message":        "Drift detected",
			})
		case errors.As(err, &cerrors.ErrNoEC2Instances{}):
			sendError(w, http.StatusBadRequest, err.Error())
		default:
			sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to check drift: %v", err))
		}
		return
	}
	sendResponse(w, http.StatusOK, map[string]interface{}{
		"drift_detected": false,
		"message":        "No drift detected",
	})
}

func sendError(w http.ResponseWriter, statusCode int, message string) {
	sendResponse(w, statusCode, map[string]interface{}{
		"error": message,
	})
}

// sendResponse sends a JSON response with the given status code and data
func sendResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
