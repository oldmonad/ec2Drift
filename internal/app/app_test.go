package app_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/oldmonad/ec2Drift/internal/app"
	"github.com/oldmonad/ec2Drift/pkg/cloud"
	"github.com/oldmonad/ec2Drift/pkg/cloud/aws"
	"github.com/oldmonad/ec2Drift/pkg/cloud/gcp"
	config "github.com/oldmonad/ec2Drift/pkg/config/cloud"
	awsConfig "github.com/oldmonad/ec2Drift/pkg/config/cloud/aws"
	"github.com/oldmonad/ec2Drift/pkg/config/env"
	customErr "github.com/oldmonad/ec2Drift/pkg/errors"
	"github.com/oldmonad/ec2Drift/pkg/logger"
	"github.com/oldmonad/ec2Drift/pkg/parser"
	"github.com/oldmonad/ec2Drift/pkg/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func createTempFile(t *testing.T, content []byte) string {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.tf")
	err := os.WriteFile(tmpFile, content, 0644)
	require.NoError(t, err)
	return tmpFile
}

func TestNewApp(t *testing.T) {
	logger.Init(true)

	// Create AWS-specific configuration
	awsConfig := &awsConfig.Config{
		AccessKey:    "test-key",
		SecretKey:    "test-secret",
		Region:       "us-west-2",
		SessionToken: "test-token",
	}

	configurations := env.Configurations{
		StatePath:         "test.tf",
		CloudProviderType: config.AWS,
		CloudConfig:       awsConfig, // Use concrete AWS config implementation
	}

	newApp := app.NewApp(configurations)
	assert.NotNil(t, newApp)
	assert.NotNil(t, newApp.Logger)
}

func TestLoadStateFileSuccess(t *testing.T) {
	content := []byte("test content")
	tmpFile := createTempFile(t, content)

	configurations := env.Configurations{
		StatePath: tmpFile,
	}
	a := app.NewApp(configurations)
	data, err := a.LoadStateFile()
	assert.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestLoadStateFileNotFound(t *testing.T) {
	configurations := env.Configurations{
		StatePath: "nonexistent.tf",
	}
	a := app.NewApp(configurations)
	_, err := a.LoadStateFile()
	require.Error(t, err)
	assert.IsType(t, customErr.ErrReadFile{}, err)
}

func TestParseConfigInstancesTerraform(t *testing.T) {
	content := []byte(`
resource "aws_instance" "test" {
  ami           = "ami-123456"
  instance_type = "t2.micro"
}`)
	configurations := env.Configurations{}
	a := app.NewApp(configurations)
	instances, err := a.ParseConfigInstances(content, parser.Terraform)

	assert.NoError(t, err)
	require.Len(t, instances, 1)
	assert.Equal(t, "ami-123456", instances[0].AMI)
	assert.Equal(t, "t2.micro", instances[0].InstanceType)
}

func TestParseConfigInstancesJSON(t *testing.T) {
	content := []byte(`[
		{
			"instance_id": "i-123456",
			"ami": "ami-789012",
			"instance_type": "t2.small"
		}
	]`)
	configurations := env.Configurations{}
	a := app.NewApp(configurations)
	instances, err := a.ParseConfigInstances(content, parser.JSON)

	assert.NoError(t, err)
	require.Len(t, instances, 1)
	assert.Equal(t, "ami-789012", instances[0].AMI)
	assert.Equal(t, "t2.small", instances[0].InstanceType)
}

func TestParseConfigInstancesInvalid(t *testing.T) {
	content := []byte(`invalid format`)
	configurations := env.Configurations{}
	a := app.NewApp(configurations)
	_, err := a.ParseConfigInstances(content, parser.Terraform)
	assert.Error(t, err)
}

func TestParseConfigInstancesDefaultParser(t *testing.T) {
	content := []byte(`
resource "aws_instance" "test" {
  ami           = "ami-123456"
  instance_type = "t2.micro"
}`)
	configurations := env.Configurations{}
	a := app.NewApp(configurations)
	instances, err := a.ParseConfigInstances(content, parser.Unknown) // Invalid parser type to trigger default

	assert.NoError(t, err)
	require.Len(t, instances, 1)
	assert.Equal(t, "ami-123456", instances[0].AMI)
}

type CloudProviderFactory func(providerType config.ProviderType) cloud.CloudProvider

func defaultCloudProviderFactory(providerType config.ProviderType) cloud.CloudProvider {
	switch providerType {
	case config.AWS:
		return &aws.AWSProvider{}
	case config.GCP:
		return &gcp.GCPProvider{}
	default:
		return &aws.AWSProvider{}
	}
}

// MockCloudProvider to mock cloud provider functionality
type MockCloudProvider struct {
	mock.Mock
}

func (m *MockCloudProvider) FetchInstances(ctx context.Context, cfg config.ProviderConfig) ([]cloud.Instance, error) {
	args := m.Called(ctx, cfg)
	return args.Get(0).([]cloud.Instance), args.Error(1)
}

// TestableApp extends App to inject mocks for testing
type TestableApp struct {
	*app.App
	mockCloudProvider *MockCloudProvider
	mockStateLoader   func() ([]byte, error)
	mockParser        func([]byte, parser.ParserType) ([]cloud.Instance, error)
}

// Create a new TestableApp with injected mocks
func NewTestableApp(cfg env.Configurations, mockProvider *MockCloudProvider) *TestableApp {
	return &TestableApp{
		App:               app.NewApp(cfg),
		mockCloudProvider: mockProvider,
	}
}

// Override GetLiveStateInstances to use mock
func (t *TestableApp) GetLiveStateInstances(ctx context.Context, cfg config.ProviderConfig) ([]cloud.Instance, error) {
	if t.mockCloudProvider != nil {
		return t.mockCloudProvider.FetchInstances(ctx, cfg)
	}
	return t.App.GetLiveStateInstances(ctx, cfg)
}

// Override LoadStateFile if mockStateLoader is provided
func (t *TestableApp) LoadStateFile() ([]byte, error) {
	if t.mockStateLoader != nil {
		return t.mockStateLoader()
	}
	return t.App.LoadStateFile()
}

// Override ParseConfigInstances if mockParser is provided
func (t *TestableApp) ParseConfigInstances(content []byte, format parser.ParserType) ([]cloud.Instance, error) {
	if t.mockParser != nil {
		return t.mockParser(content, format)
	}
	return t.App.ParseConfigInstances(content, format)
}

// Override Run to use our mocked methods
func (t *TestableApp) Run(ctx context.Context, attrs []string, format parser.ParserType, runtype ports.Runtype) error {
	// Obtain current live cloud state using mocked provider
	stateInstances, err := t.GetLiveStateInstances(ctx, t.App.Configurations().CloudConfig)
	if err != nil {
		return err
	}

	// Load desired state using mocked or real loader
	content, err := t.LoadStateFile()
	if err != nil {
		return err
	}

	// Parse desired state using mocked or real parser
	configInstances, err := t.ParseConfigInstances(content, format)
	if err != nil {
		return err
	}

	// Use the real HandleDrift method
	return t.App.HandleDrift(ctx, stateInstances, configInstances, attrs, runtype)
}

func TestRunEndToEnd(t *testing.T) {
	logger.Init(true)

	// Test case: Happy path - no drift detected
	t.Run("HappyPath_NoDrift", func(t *testing.T) {
		// Create test file content
		content := []byte(`
resource "aws_instance" "test" {
  ami           = "ami-123456"
  instance_type = "t2.micro"
}`)
		tmpFile := createTempFile(t, content)

		// Setup mock provider to return matching instances (no drift)
		mockProvider := new(MockCloudProvider)
		liveInstances := []cloud.Instance{
			{
				InstanceID:   "i-123456",
				AMI:          "ami-123456",
				InstanceType: "t2.micro",
			},
		}
		mockProvider.On("FetchInstances", mock.Anything, mock.Anything).Return(liveInstances, nil)

		// Create app with AWS config
		awsCfg := &awsConfig.Config{
			AccessKey: "test-key",
			SecretKey: "test-secret",
			Region:    "us-west-2",
		}
		configurations := env.Configurations{
			StatePath:         tmpFile,
			CloudProviderType: config.AWS,
			CloudConfig:       awsCfg,
		}

		testApp := NewTestableApp(configurations, mockProvider)
		err := testApp.Run(context.Background(), []string{"ami", "instance_type"}, parser.Terraform, ports.HTTP)

		// Verify no error returned (no drift)
		assert.NoError(t, err)
		mockProvider.AssertExpectations(t)
	})

	// Test case: Cloud provider error
	t.Run("CloudProviderError", func(t *testing.T) {
		// Create test file content
		content := []byte(`resource "aws_instance" "test" { ami = "ami-123456" }`)
		tmpFile := createTempFile(t, content)

		// Setup mock provider to return error
		mockProvider := new(MockCloudProvider)
		expectedErr := errors.New("aws api error")
		mockProvider.On("FetchInstances", mock.Anything, mock.Anything).Return([]cloud.Instance{}, expectedErr)

		// Create app config
		awsCfg := &awsConfig.Config{
			AccessKey: "test-key",
			SecretKey: "test-secret",
			Region:    "us-west-2",
		}
		configurations := env.Configurations{
			StatePath:         tmpFile,
			CloudProviderType: config.AWS,
			CloudConfig:       awsCfg,
		}

		testApp := NewTestableApp(configurations, mockProvider)
		err := testApp.Run(context.Background(), []string{"ami"}, parser.Terraform, ports.HTTP)

		// Verify provider error propagated
		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
		mockProvider.AssertExpectations(t)
	})

	// Test case: Invalid state file content (parser error)
	t.Run("ParserError", func(t *testing.T) {
		// Create invalid terraform content
		content := []byte(`this is not valid terraform or json`)
		tmpFile := createTempFile(t, content)

		// Setup mock provider
		mockProvider := new(MockCloudProvider)
		mockProvider.On("FetchInstances", mock.Anything, mock.Anything).Return([]cloud.Instance{}, nil)

		// Create app config
		awsCfg := &awsConfig.Config{
			AccessKey: "test-key",
			SecretKey: "test-secret",
			Region:    "us-west-2",
		}
		configurations := env.Configurations{
			StatePath:         tmpFile,
			CloudProviderType: config.AWS,
			CloudConfig:       awsCfg,
		}

		testApp := NewTestableApp(configurations, mockProvider)
		err := testApp.Run(context.Background(), []string{"ami"}, parser.Terraform, ports.HTTP)

		// Verify parser error returned
		assert.Error(t, err)
		// Provider should be called before parsing
		mockProvider.AssertExpectations(t)
	})

	// Test case: Multiple drifts detected
	t.Run("MultipleDriftsDetected", func(t *testing.T) {
		// Create test file with multiple attributes
		content := []byte(`
	resource "aws_instance" "test" {
	  ami           = "ami-123456"
	  instance_type = "t2.micro"
	  tags = {
	    Name        = "web-server"
	    Environment = "production"
	  }
	  root_block_device {
	    volume_size = 20
	    volume_type = "gp2"
	  }
	}`)
		tmpFile := createTempFile(t, content)

		// Setup mock provider to return instances with multiple drifts
		mockProvider := new(MockCloudProvider)
		liveInstances := []cloud.Instance{
			{
				InstanceID:   "i-123456",
				AMI:          "ami-654321", // Different AMI
				InstanceType: "t2.large",   // Different instance type
				Tags: map[string]string{
					"Name":        "web-server",
					"Environment": "staging", // Different tag value
				},
				RootBlockDevice: struct {
					VolumeSize int    `json:"volume_size"`
					VolumeType string `json:"volume_type"`
				}{
					VolumeSize: 30, // Different volume size
					VolumeType: "gp2",
				},
			},
		}
		mockProvider.On("FetchInstances", mock.Anything, mock.Anything).Return(liveInstances, nil)

		// Create app config
		awsCfg := &awsConfig.Config{
			AccessKey: "test-key",
			SecretKey: "test-secret",
			Region:    "us-west-2",
		}
		configurations := env.Configurations{
			StatePath:         tmpFile,
			CloudProviderType: config.AWS,
			CloudConfig:       awsCfg,
		}

		testApp := NewTestableApp(configurations, mockProvider)
		err := testApp.Run(context.Background(),
			[]string{"ami", "instance_type", "tags.Environment", "root_block_device.volume_size"},
			parser.Terraform,
			ports.HTTP)

		// Verify drift error returned
		require.Error(t, err)
		var driftErr customErr.ErrDriftDetected
		assert.True(t, errors.As(err, &driftErr), "expected error to be of type ErrDriftDetected")
		mockProvider.AssertExpectations(t)
	})

	// Test case: JSON format parser
	t.Run("JSONFormatParser", func(t *testing.T) {
		// Create JSON content
		content := []byte(`[
			{
				"instance_id": "i-123456",
				"ami": "ami-123456",
				"instance_type": "t2.micro"
			}
		]`)
		tmpFile := createTempFile(t, content)

		// Setup mock provider
		mockProvider := new(MockCloudProvider)
		liveInstances := []cloud.Instance{
			{
				InstanceID:   "i-123456",
				AMI:          "ami-123456",
				InstanceType: "t2.micro",
			},
		}
		mockProvider.On("FetchInstances", mock.Anything, mock.Anything).Return(liveInstances, nil)

		// Create app config
		awsCfg := &awsConfig.Config{
			AccessKey: "test-key",
			SecretKey: "test-secret",
			Region:    "us-west-2",
		}
		configurations := env.Configurations{
			StatePath:         tmpFile,
			CloudProviderType: config.AWS,
			CloudConfig:       awsCfg,
		}

		testApp := NewTestableApp(configurations, mockProvider)
		err := testApp.Run(context.Background(), []string{"ami", "instance_type"}, parser.JSON, ports.HTTP)

		// Verify no error (no drift)
		assert.NoError(t, err)
		mockProvider.AssertExpectations(t)
	})
}
