package rest_test

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/oldmonad/ec2Drift/pkg/logger"
	"github.com/oldmonad/ec2Drift/pkg/parser"
	"github.com/oldmonad/ec2Drift/pkg/ports"
	"github.com/oldmonad/ec2Drift/pkg/ports/rest"
	"go.uber.org/zap"
)

type mockAppRunner struct {
	runFunc func(ctx context.Context, attrs []string, format parser.ParserType, outputPort ports.Runtype) error
}

func (m *mockAppRunner) Run(ctx context.Context, attrs []string, format parser.ParserType, outputPort ports.Runtype) error {
	return m.runFunc(ctx, attrs, format, outputPort)
}

type mockHTTPServer struct {
	shutdownErr error
}

func (m *mockHTTPServer) ListenAndServe() error {
	return nil
}

func (m *mockHTTPServer) Shutdown(ctx context.Context) error {
	return m.shutdownErr
}

func TestHandleDriftCheckMethodNotAllowed(t *testing.T) {
	mockApp := &mockAppRunner{
		runFunc: func(ctx context.Context, attrs []string, format parser.ParserType, outputPort ports.Runtype) error {
			return nil
		},
	}

	req, err := http.NewRequest(http.MethodGet, "/drift", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rest.HandleDriftCheck(mockApp, w, r)
	})
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, status)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if errorMsg, ok := response["error"].(string); !ok || errorMsg != "Method not allowed" {
		t.Errorf("unexpected error message: %v", response["error"])
	}
}

func TestHandleDriftCheckInvalidRequestBody(t *testing.T) {
	mockApp := &mockAppRunner{
		runFunc: func(ctx context.Context, attrs []string, format parser.ParserType, outputPort ports.Runtype) error {
			return nil
		},
	}

	reqBody := strings.NewReader(`invalid json`)
	req, err := http.NewRequest(http.MethodPost, "/drift", reqBody)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rest.HandleDriftCheck(mockApp, w, r)
	})
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, status)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if errorMsg, ok := response["error"].(string); !ok || !strings.Contains(errorMsg, "Invalid request body") {
		t.Errorf("unexpected error message: %v", response["error"])
	}
}

func TestHandleDriftCheckDriftDetected(t *testing.T) {
	mockApp := &mockAppRunner{
		runFunc: func(ctx context.Context, attrs []string, format parser.ParserType, outputPort ports.Runtype) error {
			return errors.New("drift detected: instance configuration changed")
		},
	}

	reqBody := strings.NewReader(`{"attributes": ["instance_type"], "format": "terraform"}`)
	req, err := http.NewRequest(http.MethodPost, "/drift", reqBody)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rest.HandleDriftCheck(mockApp, w, r)
	})
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, status)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if drift, ok := response["drift_detected"].(bool); !ok || !drift {
		t.Error("expected drift_detected to be true")
	}
}

func TestHandleDriftCheckNoDriftDetected(t *testing.T) {
	mockApp := &mockAppRunner{
		runFunc: func(ctx context.Context, attrs []string, format parser.ParserType, outputPort ports.Runtype) error {
			return nil
		},
	}

	reqBody := strings.NewReader(`{"attributes": ["instance_type"], "format": "terraform"}`)
	req, err := http.NewRequest(http.MethodPost, "/drift", reqBody)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rest.HandleDriftCheck(mockApp, w, r)
	})
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, status)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	if drift, ok := response["drift_detected"].(bool); !ok || drift {
		t.Error("expected drift_detected to be false")
	}
}

func TestHandleDriftCheckInternalServerError(t *testing.T) {
	mockApp := &mockAppRunner{
		runFunc: func(ctx context.Context, attrs []string, format parser.ParserType, outputPort ports.Runtype) error {
			return errors.New("unexpected error")
		},
	}

	reqBody := strings.NewReader(`{}`)
	req, err := http.NewRequest(http.MethodPost, "/drift", reqBody)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rest.HandleDriftCheck(mockApp, w, r)
	})
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, status)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	expectedMsg := "Failed to check drift: unexpected error"
	if errorMsg, ok := response["error"].(string); !ok || errorMsg != expectedMsg {
		t.Errorf("expected error message '%s', got '%v'", expectedMsg, errorMsg)
	}
}

func TestHandleDriftCheckAttributesAndFormatParsing(t *testing.T) {
	var (
		calledAttrs  []string
		calledFormat parser.ParserType
	)

	mockApp := &mockAppRunner{
		runFunc: func(ctx context.Context, attrs []string, format parser.ParserType, outputPort ports.Runtype) error {
			calledAttrs = attrs
			calledFormat = format
			return nil
		},
	}

	reqBody := strings.NewReader(`{"attributes": ["attr1", "attr2"], "format": "json"}`)
	req, err := http.NewRequest(http.MethodPost, "/drift", reqBody)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rest.HandleDriftCheck(mockApp, w, r)
	})
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, status)
	}

	expectedAttrs := []string{"attr1", "attr2"}
	if !reflect.DeepEqual(calledAttrs, expectedAttrs) {
		t.Errorf("expected attributes %v, got %v", expectedAttrs, calledAttrs)
	}

	if calledFormat != parser.JSON {
		t.Errorf("expected format %v, got %v", parser.JSON, calledFormat)
	}
}

func TestHandleDriftCheckDefaultAttributesAndFormat(t *testing.T) {
	var (
		calledAttrs  []string
		calledFormat parser.ParserType
	)

	mockApp := &mockAppRunner{
		runFunc: func(ctx context.Context, attrs []string, format parser.ParserType, outputPort ports.Runtype) error {
			calledAttrs = attrs
			calledFormat = format
			return nil
		},
	}

	reqBody := strings.NewReader(`{}`)
	req, err := http.NewRequest(http.MethodPost, "/drift", reqBody)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rest.HandleDriftCheck(mockApp, w, r)
	})
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, status)
	}

	if calledAttrs == nil || len(calledAttrs) != 0 {
		t.Errorf("expected empty attributes slice, got %v", calledAttrs)
	}

	if calledFormat != parser.Terraform {
		t.Errorf("expected default format %v, got %v", parser.Terraform, calledFormat)
	}
}

func TestStartServerGracefulShutdown(t *testing.T) {
	oldLogger := logger.Log
	logger.Log = zap.NewNop()
	defer func() { logger.Log = oldLogger }()

	mockApp := &mockAppRunner{
		runFunc: func(ctx context.Context, attrs []string, format parser.ParserType, outputPort ports.Runtype) error {
			return nil
		},
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- rest.StartServer(mockApp, "0")
	}()

	time.Sleep(100 * time.Millisecond)

	proc, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("Failed to find process: %v", err)
	}
	if err := proc.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("Failed to send signal: %v", err)
	}

	select {
	case err := <-errChan:
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for server shutdown")
	}
}

func TestStartServerListenError(t *testing.T) {
	oldLogger := logger.Log
	logger.Log = zap.NewNop()
	defer func() { logger.Log = oldLogger }()

	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer listener.Close()

	mockApp := &mockAppRunner{}
	err = rest.StartServer(mockApp, "8080")

	if err == nil {
		t.Fatal("Expected error when port is in use, got nil")
	}

	if !strings.Contains(err.Error(), "address already in use") &&
		!strings.Contains(err.Error(), "Only one usage of each socket address") {
		t.Errorf("Expected address in use error, got: %v", err)
	}
}

func TestServerHandlesRequests(t *testing.T) {
	oldLogger := logger.Log
	logger.Log = zap.NewNop()
	defer func() { logger.Log = oldLogger }()

	mockApp := &mockAppRunner{
		runFunc: func(ctx context.Context, attrs []string, format parser.ParserType, outputPort ports.Runtype) error {
			return nil
		},
	}

	errChan := make(chan error, 1)
	port := "8081"

	go func() {
		errChan <- rest.StartServer(mockApp, port)
	}()

	time.Sleep(100 * time.Millisecond)

	resp, err := http.Post("http://localhost:"+port+"/drift", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %d", resp.StatusCode)
	}

	proc, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("Failed to find process: %v", err)
	}
	if err := proc.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("Failed to send signal: %v", err)
	}

	select {
	case err := <-errChan:
		if err != nil {
			t.Fatalf("Unexpected shutdown error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for server shutdown")
	}
}

func TestStartServerShutdownError(t *testing.T) {
	oldLogger := logger.Log
	logger.Log = zap.NewNop()
	defer func() { logger.Log = oldLogger }()

	mockApp := &mockAppRunner{
		runFunc: func(ctx context.Context, attrs []string, format parser.ParserType, outputPort ports.Runtype) error {
			return nil
		},
	}

	var origHttpServer interface{}
	if rest.NewHTTPServer != nil {
		origHttpServer = rest.NewHTTPServer
	}

	mockSrv := &mockHTTPServer{
		shutdownErr: errors.New("forced shutdown error"),
	}

	rest.NewHTTPServer = func(addr string, handler http.Handler) rest.HTTPServer {
		return mockSrv
	}

	defer func() {
		if origHttpServer != nil {
			rest.NewHTTPServer = origHttpServer.(func(string, http.Handler) rest.HTTPServer)
		}
	}()

	errChan := make(chan error, 1)
	go func() {
		errChan <- rest.StartServer(mockApp, "0")
	}()

	time.Sleep(100 * time.Millisecond)

	proc, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("Failed to find process: %v", err)
	}
	if err := proc.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("Failed to send signal: %v", err)
	}

	select {
	case err := <-errChan:
		if err == nil {
			t.Fatal("Expected shutdown error, got nil")
		}
		if !strings.Contains(err.Error(), "shutdown error") {
			t.Errorf("Expected 'shutdown error' in error message, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for server shutdown")
	}
}
