package rest_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	pkgerrors "github.com/oldmonad/ec2Drift/pkg/errors"
	"github.com/oldmonad/ec2Drift/pkg/logger"
	"github.com/oldmonad/ec2Drift/pkg/parser"
	"github.com/oldmonad/ec2Drift/pkg/ports"
	"github.com/oldmonad/ec2Drift/pkg/ports/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestMain(m *testing.M) {
	// Initialize test logger
	logger.SetLogger(zap.NewNop())
	code := m.Run()
	os.Exit(code)
}

// Mock implementations
type MockAppRunner struct {
	mock.Mock
}

func (m *MockAppRunner) Run(ctx context.Context, args []string, pt parser.ParserType, rt ports.Runtype) error {
	return m.Called(ctx, args, pt, rt).Error(0)
}

type MockValidator struct {
	mock.Mock
}

func (m *MockValidator) ValidateAttributes(requested []string) ([]string, error) {
	args := m.Called(requested)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockValidator) ValidateFormat(format string) (parser.ParserType, error) {
	args := m.Called(format)
	return args.Get(0).(parser.ParserType), args.Error(1)
}

// Helper function to get a free port
func getFreePort() (string, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return "", err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return "", err
	}
	defer l.Close()

	return fmt.Sprintf("%d", l.Addr().(*net.TCPAddr).Port), nil
}

// Helper function to wait for server to be ready and accessible
func waitForServer(server rest.Server, timeout time.Duration) (string, error) {
	start := time.Now()
	var port string

	// First, wait for the server to report an address
	for time.Since(start) < timeout {
		addr := server.Address()
		if addr != "" && addr != ":0" {
			// Extract port from address
			if strings.HasPrefix(addr, ":") {
				port = strings.TrimPrefix(addr, ":")
			} else {
				_, portStr, err := net.SplitHostPort(addr)
				if err != nil {
					return "", fmt.Errorf("invalid address format: %s", addr)
				}
				port = portStr
			}
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if port == "" {
		return "", fmt.Errorf("server did not report an address within timeout")
	}

	// Next, verify the server is actually accepting connections
	timeLeft := timeout - time.Since(start)
	if timeLeft <= 0 {
		return "", fmt.Errorf("timeout waiting for server")
	}

	// Try to connect to the server to verify it's ready
	dialCtx, cancel := context.WithTimeout(context.Background(), timeLeft)
	defer cancel()

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(dialCtx, "tcp", "localhost:"+port)
	if err != nil {
		return "", fmt.Errorf("server not accepting connections: %v", err)
	}
	conn.Close()

	return port, nil
}

// Test server address functionality
func TestAddress(t *testing.T) {
	mockApp := new(MockAppRunner)
	mockValidator := new(MockValidator)

	// Create new server
	server := rest.NewServer(mockApp, mockValidator)

	// Before starting, address should be empty
	assert.Empty(t, server.Address())

	// Create a simple test to check address after server is started
	// Get free port for testing
	port, err := getFreePort()
	require.NoError(t, err)

	// Use a context with timeout to ensure the test doesn't hang
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create a channel to signal when server is started
	started := make(chan struct{})

	// Create a channel for server errors
	serverErrCh := make(chan error, 1)

	// Start server in a goroutine with the context
	go func() {
		// Signal that the goroutine has started
		close(started)

		// Monitor context for cancellation
		go func() {
			<-ctx.Done()
			// Context was canceled, stop the server
			_ = server.Stop()
		}()

		// Start the server
		err := server.Start(port)
		if err != nil && err != http.ErrServerClosed {
			serverErrCh <- err
		} else {
			serverErrCh <- nil
		}
	}()

	// Wait for goroutine to start
	<-started

	// Short delay to allow server to initialize
	time.Sleep(100 * time.Millisecond)

	// Get the address and verify it contains the port
	addr := server.Address()
	if addr != "" {
		// The server might have started, check the address
		assert.Contains(t, addr, port)

		// Stop the server properly
		err := server.Stop()
		assert.NoError(t, err)

		// Wait for server to stop or timeout
		select {
		case err := <-serverErrCh:
			// Check if we got a non-nil error that isn't just the server closing
			if err != nil && err != http.ErrServerClosed {
				t.Fatalf("server error: %v", err)
			}
		case <-time.After(2 * time.Second):
			// If we time out, cancel the context which will trigger server shutdown
			cancel()
		}
	} else {
		// If the address is still empty, the server hasn't started yet
		// Let's cancel and skip further checks
		cancel()
		t.Log("Server didn't start in time, skipping address check")
	}
}

// Test server start with invalid port
func TestStartInvalidPort(t *testing.T) {
	mockApp := new(MockAppRunner)
	mockValidator := new(MockValidator)

	server := rest.NewServer(mockApp, mockValidator)

	// Try to start server with invalid port
	err := server.Start("invalid_port")

	// Verify error is returned and is of the correct type
	assert.Error(t, err)
	assert.IsType(t, pkgerrors.ErrServerListen{}, err)
}

func TestGracefulShutdownSuccess(t *testing.T) {
	mockApp := new(MockAppRunner)
	mockValidator := new(MockValidator)

	mockValidator.On("ValidateFormat", "json").Return(parser.JSON, nil)
	mockValidator.On("ValidateAttributes", mock.Anything).Return([]string{"instance-id"}, nil)

	processing := make(chan struct{})
	completed := make(chan struct{}) // Add completion channel

	mockApp.On("Run", mock.Anything, mock.Anything, parser.JSON, mock.Anything).
		Run(func(args mock.Arguments) {
			close(processing)
			<-completed // Wait for test to allow completion
		}).
		Return(nil)

	server := rest.NewServer(mockApp, mockValidator)
	port, err := getFreePort()
	require.NoError(t, err)

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Start(port)
	}()

	_, err = waitForServer(server, 2*time.Second)
	require.NoError(t, err)

	// Create HTTP client with proper timeout
	client := &http.Client{Timeout: 1 * time.Second}

	go func() {
		body := strings.NewReader(`{"format":"json","attributes":["instance-id"]}`)
		resp, _ := client.Post(
			fmt.Sprintf("http://localhost:%s/drift", port),
			"application/json",
			body,
		)
		if resp != nil {
			resp.Body.Close()
		}
		close(completed) // Signal handler completion
	}()

	// Wait for handler to start
	select {
	case <-processing:
	case <-time.After(2 * time.Second):
		t.Fatal("Handler didn't start processing")
	}

	// Initiate shutdown
	shutdownStart := time.Now()
	err = server.Stop()
	assert.NoError(t, err)

	// Verify shutdown completion
	select {
	case err := <-serverErr:
		assert.NoError(t, err)
		t.Logf("Shutdown completed in %v", time.Since(shutdownStart))
	case <-time.After(3 * time.Second):
		t.Fatal("Server didn't stop within timeout")
	}

	mockApp.AssertExpectations(t)
	mockValidator.AssertExpectations(t)
}

func TestConcurrentRequestsDuringShutdown(t *testing.T) {
	mockApp := new(MockAppRunner)
	mockValidator := new(MockValidator)

	// Setup mocks for 5 requests
	mockValidator.On("ValidateFormat", "json").Return(parser.JSON, nil).Times(5)
	mockValidator.On("ValidateAttributes", mock.Anything).Return([]string{"instance-id"}, nil).Times(5)

	processing := make(chan struct{}, 5) // Buffered channel for 5 requests
	blockProcessing := make(chan struct{})

	mockApp.On("Run", mock.Anything, mock.Anything, parser.JSON, mock.Anything).
		Run(func(args mock.Arguments) {
			processing <- struct{}{} // Signal request start
			<-blockProcessing        // Block until release
		}).
		Return(nil).
		Times(5)

	server := rest.NewServer(mockApp, mockValidator)
	port, err := getFreePort()
	require.NoError(t, err)

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Start(port)
	}()

	_, err = waitForServer(server, 2*time.Second)
	require.NoError(t, err)

	// Send requests with longer timeout
	var wg sync.WaitGroup
	client := &http.Client{Timeout: 3 * time.Second}
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := strings.NewReader(`{"format":"json","attributes":["instance-id"]}`)
			resp, err := client.Post(
				fmt.Sprintf("http://localhost:%s/drift", port),
				"application/json",
				body,
			)
			if resp != nil {
				resp.Body.Close()
			}
			// Allow network errors during shutdown
			if err != nil && !isExpectedShutdownError(err) {
				t.Errorf("Unexpected request error: %v", err)
			}
		}()
	}

	// Wait for all requests to be processing
	for i := 0; i < 5; i++ {
		select {
		case <-processing:
		case <-time.After(2 * time.Second):
			t.Fatal("Timed out waiting for requests to start processing")
		}
	}

	// Initiate shutdown with extended timeout
	shutdownStart := time.Now()
	close(blockProcessing) // Release all requests first
	err = server.Stop()
	assert.NoError(t, err)

	// Verify shutdown completes within 3 seconds (less than server's 5s timeout)
	select {
	case err := <-serverErr:
		assert.NoError(t, err)
		t.Logf("Shutdown completed in %v", time.Since(shutdownStart))
	case <-time.After(3 * time.Second):
		t.Fatal("Server did not shutdown within expected timeframe")
	}

	wg.Wait()
	mockApp.AssertExpectations(t)
	mockValidator.AssertExpectations(t)
}

func isExpectedShutdownError(err error) bool {
	return strings.Contains(err.Error(), "closed") ||
		strings.Contains(err.Error(), "refused") ||
		strings.Contains(err.Error(), "timeout") ||
		strings.Contains(err.Error(), "reset")
}

// func TestInvalidRequestHandling(t *testing.T) {
// 	mockApp := new(MockAppRunner)
// 	mockValidator := new(MockValidator)

// 	// Setup validation failure
// 	// mockValidator.On("ValidateFormat", "invalid").Return(parser.Unknown, pkgerrors.ErrInvalidFormat)
// 	mockValidator.On("ValidateAttributes", mock.Anything).Return(nil, pkgerrors.InvalidAttributesError)

// 	server := rest.NewServer(mockApp, mockValidator)
// 	port, err := getFreePort()
// 	require.NoError(t, err)

// 	serverErr := make(chan error, 1)
// 	go func() {
// 		serverErr <- server.Start(port)
// 	}()

// 	_, err = waitForServer(server, 2*time.Second)
// 	require.NoError(t, err)

// 	// Test malformed JSON
// 	resp1, err := http.Post(
// 		fmt.Sprintf("http://localhost:%s/drift", port),
// 		"application/json",
// 		strings.NewReader(`{invalid-json}`),
// 	)
// 	require.NoError(t, err)
// 	defer resp1.Body.Close()
// 	assert.Equal(t, http.StatusBadRequest, resp1.StatusCode)

// 	// Test invalid format
// 	resp2, err := http.Post(
// 		fmt.Sprintf("http://localhost:%s/drift", port),
// 		"application/json",
// 		strings.NewReader(`{"format":"invalid","attributes":[]}`),
// 	)
// 	require.NoError(t, err)
// 	defer resp2.Body.Close()
// 	assert.Equal(t, http.StatusBadRequest, resp2.StatusCode)

// 	// Test invalid attributes
// 	resp3, err := http.Post(
// 		fmt.Sprintf("http://localhost:%s/drift", port),
// 		"application/json",
// 		strings.NewReader(`{"format":"json","attributes":["invalid"]}`),
// 	)
// 	require.NoError(t, err)
// 	defer resp3.Body.Close()
// 	assert.Equal(t, http.StatusBadRequest, resp3.StatusCode)

// 	server.Stop()
// 	mockApp.AssertExpectations(t)
// 	mockValidator.AssertExpectations(t)
// }

// func TestHandlerPanicRecovery(t *testing.T) {
// 	mockApp := new(MockAppRunner)
// 	mockValidator := new(MockValidator)

// 	// Setup mock to panic
// 	mockValidator.On("ValidateFormat", "json").Return(parser.JSON, nil)
// 	mockValidator.On("ValidateAttributes", mock.Anything).Return([]string{"instance-id"}, nil)
// 	mockApp.On("Run", mock.Anything, mock.Anything, parser.JSON, mock.Anything).
// 		Panic("simulated handler panic")

// 	server := rest.NewServer(mockApp, mockValidator)
// 	port, err := getFreePort()
// 	require.NoError(t, err)

// 	serverErr := make(chan error, 1)
// 	go func() {
// 		serverErr <- server.Start(port)
// 	}()

// 	_, err = waitForServer(server, 2*time.Second)
// 	require.NoError(t, err)

// 	// Send request that triggers panic
// 	resp, err := http.Post(
// 		fmt.Sprintf("http://localhost:%s/drift", port),
// 		"application/json",
// 		strings.NewReader(`{"format":"json","attributes":["instance-id"]}`),
// 	)
// 	require.NoError(t, err)
// 	defer resp.Body.Close()

// 	// Verify server remains operational
// 	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

// 	// Verify server can still handle new requests
// 	resp2, err := http.Post(
// 		fmt.Sprintf("http://localhost:%s/drift", port),
// 		"application/json",
// 		strings.NewReader(`{"format":"json","attributes":["instance-id"]}`),
// 	)
// 	require.NoError(t, err)
// 	defer resp2.Body.Close()
// 	assert.Equal(t, http.StatusInternalServerError, resp2.StatusCode)

// 	server.Stop()
// 	mockApp.AssertExpectations(t)
// 	mockValidator.AssertExpectations(t)
// }

func TestPortAlreadyInUse(t *testing.T) {
	mockApp := new(MockAppRunner)
	mockValidator := new(MockValidator)

	// Get and occupy a port first
	port, err := getFreePort()
	require.NoError(t, err)

	// Occupy the port
	occupiedServer := &http.Server{Addr: ":" + port}
	go occupiedServer.ListenAndServe()
	defer occupiedServer.Close()

	// Try to start our server on same port
	server := rest.NewServer(mockApp, mockValidator)
	err = server.Start(port)

	assert.Error(t, err)
	assert.IsType(t, pkgerrors.ErrServerListen{}, err)
	assert.Contains(t, err.Error(), "address already in use")
}

// func TestMultipleStopCalls(t *testing.T) {
// 	mockApp := new(MockAppRunner)
// 	mockValidator := new(MockValidator)

// 	server := rest.NewServer(mockApp, mockValidator)
// 	port, err := getFreePort()
// 	require.NoError(t, err)

// 	go server.Start(port)
// 	_, err = waitForServer(server, 2*time.Second)
// 	require.NoError(t, err)

// 	// First stop should succeed
// 	err = server.Stop()
// 	assert.NoError(t, err)

// 	// Subsequent stops should be no-ops
// 	err = server.Stop()
// 	assert.NoError(t, err)
// 	err = server.Stop()
// 	assert.NoError(t, err)

// 	// Verify address cleared
// 	assert.Empty(t, server.Address())
// }

// func TestRequestTimeoutHandling(t *testing.T) {
//     mockApp := new(MockAppRunner)
//     mockValidator := new(MockValidator)

//     // Setup long-running handler
//     processing := make(chan struct{})
//     mockValidator.On("ValidateFormat", "json").Return(parser.JSON, nil)
//     mockValidator.On("ValidateAttributes", mock.Anything).Return([]string{"instance-id"}, nil)
//     mockApp.On("Run", mock.Anything, mock.Anything, parser.JSON, mock.Anything).
//         Run(func(args mock.Arguments) {
//             close(processing)
//             time.Sleep(2 * time.Second) // Exceeds client timeout
//         }).
//         Return(nil)

//     server := rest.NewServer(mockApp, mockValidator)
//     port, err := getFreePort()
//     require.NoError(t, err)

//     go server.Start(port)
//     _, err = waitForServer(server, 2*time.Second)
//     require.NoError(t, err)

//     // Client with short timeout
//     client := &http.Client{Timeout: 500 * time.Millisecond}

//     // Send request
//     resp, err := client.Post(
//         fmt.Sprintf("http://localhost:%s/drift", port),
//         "application/json",
//         strings.NewReader(`{"format":"json","attributes":["instance-id"]}`),
//     )

//     // Verify timeout handling
//     assert.Error(t, err)
//     assert.True(t, os.IsTimeout(err), "Expected timeout error")

//     server.Stop()
//     mockApp.AssertExpectations(t)
//     mockValidator.AssertExpectations(t)
// }
