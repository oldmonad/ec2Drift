package env_test

import (
	"errors"
	"testing"

	"github.com/oldmonad/ec2Drift/pkg/config/cloud"
	"github.com/oldmonad/ec2Drift/pkg/config/env"
	err "github.com/oldmonad/ec2Drift/pkg/errors"
	"github.com/oldmonad/ec2Drift/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// Mock AWS Config for testing
type MockAWSConfig struct {
	mock.Mock
}

func (m *MockAWSConfig) Validate() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockAWSConfig) GetCredentials() interface{} {
	args := m.Called()
	return args.Get(0)
}

func (m *MockAWSConfig) GetRegion() string {
	args := m.Called()
	return args.String(0)
}

// Mock GCP Config for testing
type MockGCPConfig struct {
	mock.Mock
}

func (m *MockGCPConfig) Validate() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockGCPConfig) GetCredentials() interface{} {
	args := m.Called()
	return args.Get(0)
}

func (m *MockGCPConfig) GetRegion() string {
	args := m.Called()
	return args.String(0)
}

type MockProviderConfigFactory struct {
	mock.Mock
}

func (m *MockProviderConfigFactory) NewProviderConfig(provider cloud.ProviderType) (cloud.ProviderConfig, error) {
	args := m.Called(provider)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(cloud.ProviderConfig), args.Error(1)
}

func TestMain(m *testing.M) {
	logger.SetLogger(zap.NewNop())
	m.Run()
}

func TestLoadGeneralConfig(t *testing.T) {
	tests := []struct {
		name           string
		env            map[string]string
		expectedConfig *env.Configurations
		expectErr      bool
		errType        interface{}
	}{
		{
			name: "all fields valid",
			env: map[string]string{
				"DEBUG":          "true",
				"LOG_LEVEL":      "debug",
				"CONFIG_PATH":    "/config",
				"STATE_PATH":     "/state",
				"OUTPUT_PATH":    "/output",
				"HTTP_PORT":      "8081",
				"CLOUD_PROVIDER": "aws",
			},
			expectedConfig: &env.Configurations{
				DebugMode:         true,
				LogLevel:          "debug",
				ConfigPath:        "/config",
				StatePath:         "/state",
				OutputPath:        "/output",
				HttpPort:          8081,
				CloudProviderType: "aws",
			},
			expectErr: false,
		},
		{
			name: "invalid DEBUG",
			env: map[string]string{
				"DEBUG":          "maybe",
				"CLOUD_PROVIDER": "aws",
			},
			expectedConfig: &env.Configurations{
				DebugMode:         false,
				HttpPort:          8080,
				CloudProviderType: "",
			},
			expectErr: true,
			errType:   &err.ErrDebugParse{},
		},
		{
			name: "invalid HTTP_PORT",
			env: map[string]string{
				"DEBUG":          "true",
				"LOG_LEVEL":      "info",
				"HTTP_PORT":      "invalid",
				"CLOUD_PROVIDER": "aws",
			},
			expectedConfig: &env.Configurations{
				DebugMode:         true,
				LogLevel:          "info",
				HttpPort:          8080,
				CloudProviderType: "",
			},
			expectErr: true,
			errType:   &err.ErrPortParse{},
		},
		{
			name: "HTTP_PORT out of range",
			env: map[string]string{
				"DEBUG":          "true",
				"HTTP_PORT":      "0",
				"CLOUD_PROVIDER": "aws",
			},
			expectedConfig: &env.Configurations{
				DebugMode:         true,
				HttpPort:          8080,
				CloudProviderType: "",
			},
			expectErr: true,
			errType:   &err.ErrPortOutOfRange{},
		},
		{
			name: "missing CLOUD_PROVIDER",
			env: map[string]string{
				"DEBUG":     "true",
				"HTTP_PORT": "8081",
			},
			expectedConfig: &env.Configurations{
				DebugMode:         true,
				HttpPort:          8081,
				CloudProviderType: "",
			},
			expectErr: true,
			errType:   &err.ErrMissingCloudProvider{},
		},
		{
			name: "HTTP_PORT default",
			env: map[string]string{
				"DEBUG":          "true",
				"CLOUD_PROVIDER": "aws",
			},
			expectedConfig: &env.Configurations{
				DebugMode:         true,
				HttpPort:          8080,
				CloudProviderType: "aws",
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			defer func() {
				for k := range tt.env {
					t.Setenv(k, "")
				}
			}()

			cfg := env.NewConfiguration()
			err := cfg.LoadGeneralConfig()

			if tt.expectErr {
				assert.Error(t, err)
				if tt.errType != nil {
					assert.ErrorAs(t, err, tt.errType)
				}
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedConfig.DebugMode, cfg.DebugMode)
			assert.Equal(t, tt.expectedConfig.LogLevel, cfg.LogLevel)
			assert.Equal(t, tt.expectedConfig.ConfigPath, cfg.ConfigPath)
			assert.Equal(t, tt.expectedConfig.StatePath, cfg.StatePath)
			assert.Equal(t, tt.expectedConfig.OutputPath, cfg.OutputPath)
			assert.Equal(t, tt.expectedConfig.HttpPort, cfg.HttpPort)
			assert.Equal(t, tt.expectedConfig.CloudProviderType, cfg.CloudProviderType)
		})
	}
}

func TestValidateAndSetPort(t *testing.T) {
	tests := []struct {
		name          string
		envPort       string
		expectedPort  int
		expectedError interface{}
	}{
		{
			name:          "valid port",
			envPort:       "8081",
			expectedPort:  8081,
			expectedError: nil,
		},
		{
			name:          "empty port uses default",
			envPort:       "",
			expectedPort:  8080,
			expectedError: nil,
		},
		{
			name:          "invalid port",
			envPort:       "invalid",
			expectedPort:  8080,
			expectedError: &err.ErrPortParse{},
		},
		{
			name:          "port too low",
			envPort:       "0",
			expectedPort:  8080,
			expectedError: &err.ErrPortOutOfRange{},
		},
		{
			name:          "port too high",
			envPort:       "65536",
			expectedPort:  8080,
			expectedError: &err.ErrPortOutOfRange{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HTTP_PORT", tt.envPort)
			defer t.Setenv("HTTP_PORT", "")

			cfg := env.NewConfiguration()
			err := cfg.ValidateAndSetPort()

			if tt.expectedError != nil {
				assert.ErrorAs(t, err, &tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedPort, cfg.HttpPort)
		})
	}
}

func TestLoadCloudConfig(t *testing.T) {
	tests := []struct {
		name      string
		provider  cloud.ProviderType
		mockSetup func(*MockProviderConfigFactory)
		expectErr bool
		// expectedErr error
		expectedErr string
	}{
		{
			name:     "successful config load - AWS",
			provider: "aws",
			mockSetup: func(m *MockProviderConfigFactory) {
				mockConfig := new(MockAWSConfig)
				m.On("NewProviderConfig", cloud.ProviderType("aws")).Return(mockConfig, nil)
			},
			expectErr: false,
		},
		{
			name:     "successful config load - GCP",
			provider: "gcp",
			mockSetup: func(m *MockProviderConfigFactory) {
				mockConfig := new(MockGCPConfig)
				m.On("NewProviderConfig", cloud.ProviderType("gcp")).Return(mockConfig, nil)
			},
			expectErr: false,
		},
		{
			name:     "error creating provider config",
			provider: "unknown",
			mockSetup: func(m *MockProviderConfigFactory) {
				m.On("NewProviderConfig", cloud.ProviderType("unknown")).Return(
					nil, err.NewUnsupportedProvider("unknown"))
			},
			expectErr:   true,
			expectedErr: "unsupported provider: unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFactory := new(MockProviderConfigFactory)
			tt.mockSetup(mockFactory)

			cfg := env.NewConfiguration()
			cfg.CloudProviderType = tt.provider
			cfg.CloudProvider = mockFactory

			err := cfg.LoadCloudConfig()

			mockFactory.AssertExpectations(t)

			if tt.expectErr {
				assert.Error(t, err)
				// We know this is a wrapped error, but we can't check the specific type
				// as err.ErrLoadCloudConfig is not exported as a type
				// assert.Contains(t, err.Error(), "failed to load cloud config")
				assert.EqualError(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cfg.CloudConfig)
			}
		})
	}
}

// Tests for general configuration validator
func TestValidateGeneralConfig(t *testing.T) {
	tests := []struct {
		name            string
		statePath       string
		outputPath      string
		cloudConfig     cloud.ProviderConfig
		validateReturns error
		expectErr       bool
		expectedErrType interface{}
	}{
		{
			name:            "valid configuration",
			statePath:       "/state",
			outputPath:      "/output",
			cloudConfig:     &MockAWSConfig{},
			validateReturns: nil,
			expectErr:       false,
		},
		{
			name:            "missing state path",
			statePath:       "",
			outputPath:      "/output",
			cloudConfig:     &MockAWSConfig{},
			validateReturns: nil,
			expectErr:       true,
			expectedErrType: &err.ErrMissingPaths{},
		},
		{
			name:            "missing output path",
			statePath:       "/state",
			outputPath:      "",
			cloudConfig:     &MockAWSConfig{},
			validateReturns: nil,
			expectErr:       false,
			expectedErrType: nil,
		},
		{
			name:            "nil cloud config",
			statePath:       "/state",
			outputPath:      "/output",
			cloudConfig:     nil,
			validateReturns: nil,
			expectErr:       true,
			expectedErrType: &err.ErrCloudConfigNotInit{},
		},
		{
			name:            "cloud config validation error",
			statePath:       "/state",
			outputPath:      "/output",
			cloudConfig:     &MockAWSConfig{},
			validateReturns: errors.New("validation failed"),
			expectErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := env.NewConfiguration()
			cfg.StatePath = tt.statePath
			cfg.OutputPath = tt.outputPath
			cfg.CloudConfig = tt.cloudConfig

			// Only set up the mock expectation if we expect to reach that code
			if tt.cloudConfig != nil && tt.statePath != "" {
				if mockConfig, ok := tt.cloudConfig.(*MockAWSConfig); ok {
					mockConfig.On("Validate").Return(tt.validateReturns)
				}
			}

			err := cfg.ValidateGeneralConfig()

			if tt.expectErr {
				assert.Error(t, err)
				// For errors that we can't check the exact type due to not being exported,
				// we can check the error message instead
				if tt.name == "LoadGeneralConfig error" {
					assert.Contains(t, err.Error(), "failed to load general config")
				}
				if tt.name == "LoadCloudConfig error" {
					assert.Contains(t, err.Error(), "failed to load cloud config")
				}
				if tt.name == "ValidateGeneralConfig error" {
					assert.Contains(t, err.Error(), "invalid configurations")
				}
			} else {
				assert.NoError(t, err)
			}

			// Only verify mock expectations if the mock should have been called
			if mockConfig, ok := tt.cloudConfig.(*MockAWSConfig); ok && tt.statePath != "" && tt.outputPath != "" {
				mockConfig.AssertExpectations(t)
			}
		})
	}
}

func TestPortToString(t *testing.T) {
	tests := []struct {
		name        string
		port        int
		expectedStr string
	}{
		{
			name:        "default port",
			port:        8080,
			expectedStr: "8080",
		},
		{
			name:        "custom port",
			port:        9000,
			expectedStr: "9000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := env.NewConfiguration()
			cfg.HttpPort = tt.port

			result := cfg.PortToString()

			assert.Equal(t, tt.expectedStr, result)
		})
	}
}

// Test for InitiateLogger - just ensure it doesn't panic
func TestInitiateLogger(t *testing.T) {
	tests := []struct {
		name      string
		debugMode bool
	}{
		{
			name:      "debug mode true",
			debugMode: true,
		},
		{
			name:      "debug mode false",
			debugMode: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := env.NewConfiguration()
			cfg.DebugMode = tt.debugMode

			// Just verify it doesn't panic
			assert.NotPanics(t, func() {
				cfg.InitiateLogger()
			})
		})
	}
}

func TestSetupConfigurationsLoadGeneralConfigError(t *testing.T) {
	t.Setenv("DEBUG", "true")
	t.Setenv("STATE_PATH", "/state")
	t.Setenv("OUTPUT_PATH", "/output")
	// Missing CLOUD_PROVIDER

	_, setupErr := env.SetupConfigurations()
	assert.Error(t, setupErr)

	var missingProv err.ErrMissingCloudProvider
	assert.ErrorAs(t, setupErr, &missingProv, "should be ErrMissingCloudProvider")
}

func TestSetupConfigurationsLoadCloudConfigError(t *testing.T) {
	t.Setenv("DEBUG", "true")
	t.Setenv("CLOUD_PROVIDER", "invalid-provider")
	t.Setenv("STATE_PATH", "/state")
	t.Setenv("OUTPUT_PATH", "/output")

	_, setupErr := env.SetupConfigurations()
	assert.Error(t, setupErr)

	var unsupported err.ErrUnsupportedProvider
	assert.ErrorAs(t, setupErr, &unsupported, "error should be ErrUnsupportedProvider")
	assert.EqualError(t, unsupported, "unsupported provider: invalid-provider")
}
