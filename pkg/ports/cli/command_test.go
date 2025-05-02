package cli_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/oldmonad/ec2Drift/pkg/config/env"
	"github.com/oldmonad/ec2Drift/pkg/parser"
	"github.com/oldmonad/ec2Drift/pkg/ports"
	"github.com/oldmonad/ec2Drift/pkg/ports/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock AppRunner simulates the application runner for testing purposes
type MockAppRunner struct {
	mock.Mock
}

// Run simulates the Run method of the application runner
func (m *MockAppRunner) Run(ctx context.Context, attrs []string, format parser.ParserType, output ports.Runtype) error {
	args := m.Called(ctx, attrs, format, output)
	return args.Error(0)
}

// Mock Validator simulates the validator for testing purposes
type MockValidator struct {
	mock.Mock
}

// ValidateFormat simulates validating the format input
func (m *MockValidator) ValidateFormat(format string) (parser.ParserType, error) {
	args := m.Called(format)
	return args.Get(0).(parser.ParserType), args.Error(1)
}

// ValidateAttributes simulates validating the attributes input
func (m *MockValidator) ValidateAttributes(attrs []string) ([]string, error) {
	args := m.Called(attrs)
	return args.Get(0).([]string), args.Error(1)
}

// Mock Server simulates the server for testing purposes
type MockServer struct {
	mock.Mock
}

// Start simulates starting the server
func (m *MockServer) Start(port string) error {
	args := m.Called(port)
	return args.Error(0)
}

// Stop simulates stopping the server
func (m *MockServer) Stop() error {
	args := m.Called()
	return args.Error(0)
}

// Address simulates getting the server's address
func (m *MockServer) Address() string {
	args := m.Called()
	return args.String(0)
}

// TestEnvConfigurations simulates the environment configuration for testing purposes
type TestEnvConfigurations struct {
	*env.Configurations // Embed the actual type
	PortToStringFunc    func() string
	InitiateLoggerFunc  func()
}

// Override the methods we want to mock in the test configurations
func (t *TestEnvConfigurations) PortToString() string {
	if t.PortToStringFunc != nil {
		return t.PortToStringFunc()
	}
	return t.Configurations.PortToString() // Fallback to real implementation
}

func (t *TestEnvConfigurations) InitiateLogger() {
	if t.InitiateLoggerFunc != nil {
		t.InitiateLoggerFunc()
		return
	}
	t.Configurations.InitiateLogger() // Fallback to real implementation
}

// NewTestEnvConfigurations returns a new instance of the testable environment configurations
func NewTestEnvConfigurations() *TestEnvConfigurations {
	realConfigs := env.NewConfiguration()

	// Set default test values
	realConfigs.HttpPort = 8080

	return &TestEnvConfigurations{
		Configurations: realConfigs,
	}
}

// TestInitiateCommands tests the initialization of commands
func TestInitiateCommands(t *testing.T) {
	// Create test env with mockable methods
	testEnv := NewTestEnvConfigurations()

	// Create the mock server
	mockServer := new(MockServer)

	// Create the command using our testable env configurations
	cmd := cli.NewCommand(
		new(MockAppRunner),
		new(MockValidator),
		mockServer,
		testEnv.Configurations,
	)

	// Initiate root command and verify its structure
	rootCmd := cmd.InitiateCommands()
	assert.Equal(t, "ec2drift", rootCmd.Use)
	assert.Len(t, rootCmd.Commands(), 2)
	assert.Equal(t, "run", rootCmd.Commands()[0].Use)
	assert.Equal(t, "serve", rootCmd.Commands()[1].Use)
}

// TestRunCommandSuccess tests the successful execution of the "run" command
func TestRunCommandSuccess(t *testing.T) {
	mockApp := new(MockAppRunner)
	mockValidator := new(MockValidator)
	testEnv := NewTestEnvConfigurations()

	// Set up validator mock expectations
	mockValidator.On("ValidateFormat", "terraform").Return(parser.ParserType("terraform"), nil)
	mockValidator.On("ValidateAttributes", []string{"attr1"}).Return([]string{"valid_attr1"}, nil)

	// Set up app runner mock expectations
	mockApp.On("Run", mock.Anything, []string{"valid_attr1"}, parser.ParserType("terraform"), ports.CLI).Return(nil)

	// Create command and initiate root command
	cmd := cli.NewCommand(
		mockApp,
		mockValidator,
		new(MockServer),
		testEnv.Configurations,
	)
	rootCmd := cmd.InitiateCommands()

	// Set args to run the command with flags
	rootCmd.SetArgs([]string{"run", "--format", "terraform", "--attributes", "attr1"})

	// Execute the root command
	err := rootCmd.Execute()

	// Assert no error and verify mocks
	assert.NoError(t, err)
	mockValidator.AssertExpectations(t)
	mockApp.AssertExpectations(t)
}

// TestRunCommandInvalidFormat tests the behavior of the "run" command when provided with an invalid format
func TestRunCommandInvalidFormat(t *testing.T) {
	mockApp := new(MockAppRunner)
	mockValidator := new(MockValidator)
	testEnv := NewTestEnvConfigurations()

	// Set up validator mock expectation for invalid format
	expectedError := errors.New("invalid format specified")
	mockValidator.On("ValidateFormat", "invalid-format").Return(parser.ParserType(""), expectedError)

	// Create command and set invalid format in args
	cmd := cli.NewCommand(
		mockApp,
		mockValidator,
		new(MockServer),
		testEnv.Configurations,
	)
	rootCmd := cmd.InitiateCommands()
	rootCmd.SetArgs([]string{"run", "--format", "invalid-format", "--attributes", "attr1"})

	// Execute and capture error
	err := rootCmd.Execute()
	cleanedErr := cleanCobraError(err)

	// Assert error message is as expected
	assert.Contains(t, cleanedErr, "invalid format specified")
	mockValidator.AssertExpectations(t)
	mockApp.AssertNotCalled(t, "Run", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// TestServeCommandSuccess tests the successful execution of the "serve" command
func TestServeCommandSuccess(t *testing.T) {
	mockApp := new(MockAppRunner)
	mockValidator := new(MockValidator)
	mockServer := new(MockServer)
	testEnv := NewTestEnvConfigurations()

	// Set up server mock with expected port
	expectedPort := testEnv.PortToString()
	mockServer.On("Start", expectedPort).Return(nil)

	// Create command and initiate root command
	cmd := cli.NewCommand(
		mockApp,
		mockValidator,
		mockServer,
		testEnv.Configurations,
	)
	rootCmd := cmd.InitiateCommands()
	rootCmd.SetArgs([]string{"serve"})

	// Execute the command and assert no error
	err := rootCmd.Execute()
	assert.NoError(t, err)

	// Verify server start call and no unexpected interactions with other components
	mockServer.AssertCalled(t, "Start", expectedPort)
	mockServer.AssertNotCalled(t, "Stop")
	mockApp.AssertNotCalled(t, "Run")
	mockValidator.AssertNotCalled(t, "ValidateFormat")
	mockValidator.AssertNotCalled(t, "ValidateAttributes")
	mockServer.AssertNumberOfCalls(t, "Start", 1)
}

// TestServeCommandPortError tests the "serve" command when there is a port error
func TestServeCommandPortError(t *testing.T) {
	mockApp := new(MockAppRunner)
	mockValidator := new(MockValidator)
	mockServer := new(MockServer)
	testEnv := NewTestEnvConfigurations()

	// Set up expected error for server start failure
	expectedPort := testEnv.PortToString()
	expectedError := errors.New("port 8080 already in use")
	mockServer.On("Start", expectedPort).Return(expectedError)

	// Create command and initiate root command
	cmd := cli.NewCommand(
		mockApp,
		mockValidator,
		mockServer,
		testEnv.Configurations,
	)
	rootCmd := cmd.InitiateCommands()
	rootCmd.SetArgs([]string{"serve"})

	// Execute the command and assert error
	err := rootCmd.Execute()
	assert.Error(t, err)
	assert.EqualError(t, err, expectedError.Error())

	// Verify mock interactions and ensure no other components are involved
	mockServer.AssertCalled(t, "Start", expectedPort)
	mockServer.AssertNotCalled(t, "Stop")
	mockApp.AssertNotCalled(t, "Run")
	mockValidator.AssertNotCalled(t, "ValidateFormat")
	mockValidator.AssertNotCalled(t, "ValidateAttributes")
	mockServer.AssertNumberOfCalls(t, "Start", 1)
}

// TestRunCommandInvalidAttributes tests the "run" command when invalid attributes are provided
func TestRunCommandInvalidAttributes(t *testing.T) {
	mockApp := new(MockAppRunner)
	mockValidator := new(MockValidator)
	testEnv := NewTestEnvConfigurations()

	// Set up valid format and invalid attributes
	mockValidator.On("ValidateFormat", "terraform").Return(parser.ParserType("terraform"), nil)
	invalidAttrs := []string{"invalid_attr1", "invalid_attr2"}
	expectedError := errors.New("invalid attributes: invalid_attr1, invalid_attr2")
	mockValidator.On("ValidateAttributes", invalidAttrs).Return([]string{}, expectedError)

	// Create command and initiate root command
	cmd := cli.NewCommand(
		mockApp,
		mockValidator,
		new(MockServer),
		testEnv.Configurations,
	)
	rootCmd := cmd.InitiateCommands()
	rootCmd.SetArgs([]string{
		"run",
		"--format", "terraform",
		"--attributes", strings.Join(invalidAttrs, ","),
	})

	// Execute the command and assert error
	err := rootCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid attributes: invalid_attr1, invalid_attr2")

	// Verify mock interactions
	mockValidator.AssertExpectations(t)
	mockApp.AssertNotCalled(t, "Run")
}

// cleanCobraError cleans up the error message returned by Cobra command execution
func cleanCobraError(err error) string {
	if err == nil {
		return ""
	}
	parts := strings.SplitN(err.Error(), "\nUsage:", 2)
	return strings.TrimSpace(parts[0])
}
