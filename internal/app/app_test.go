package app_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/oldmonad/ec2Drift/internal/app"
	"github.com/oldmonad/ec2Drift/pkg/cloud"
	config "github.com/oldmonad/ec2Drift/pkg/config/cloud"
	"github.com/oldmonad/ec2Drift/pkg/config/env"
	customErr "github.com/oldmonad/ec2Drift/pkg/errors"
	"github.com/oldmonad/ec2Drift/pkg/logger"
	"github.com/oldmonad/ec2Drift/pkg/parser"
	"github.com/oldmonad/ec2Drift/pkg/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockCloudProvider struct {
	mock.Mock
}

func (m *MockCloudProvider) FetchInstances(ctx context.Context, cfg config.ProviderConfig) ([]cloud.Instance, error) {
	args := m.Called(ctx, cfg)
	return args.Get(0).([]cloud.Instance), args.Error(1)
}

type MockParser struct {
	mock.Mock
}

func (m *MockParser) Parse(content []byte) ([]cloud.Instance, error) {
	args := m.Called(content)
	return args.Get(0).([]cloud.Instance), args.Error(1)
}

func TestNewApp(t *testing.T) {
	logger.Init(true)
	generalCfg := env.GeneralConfig{StatePath: "test.json"}
	newApp := app.New(generalCfg)
	assert.NotNil(t, newApp)
	assert.NotNil(t, newApp.Logger)
}

func TestRunStateFileNotFound(t *testing.T) {
	app := app.New(env.GeneralConfig{StatePath: "nonexistent.json"})
	err := app.Run(context.Background(), nil, parser.Terraform, ports.HTTP)
	require.Error(t, err)

	var readErr customErr.ErrReadFile
	assert.True(t, errors.As(err, &readErr), "expected error to be of type ErrReadFile")
}

func TestRunWithInvalidCloudProvider(t *testing.T) {
	content := []byte(`provider "aws" {}`)
	tmpFile := createTempFile(t, content)

	oldProvider := os.Getenv("CLOUD_PROVIDER")
	defer os.Setenv("CLOUD_PROVIDER", oldProvider)
	os.Setenv("CLOUD_PROVIDER", "invalid_provider")

	testApp := app.New(env.GeneralConfig{StatePath: tmpFile})
	err := testApp.Run(context.Background(), nil, parser.Terraform, ports.HTTP)

	var providerErr customErr.ErrInvalidCloudProvider
	require.ErrorAs(t, err, &providerErr)
	assert.Equal(t, "invalid_provider", providerErr.Provider)
}

func TestRunWithDriftDetected(t *testing.T) {
	content := []byte(`
provider "aws" {
  region = "us-west-2"
}
resource "aws_instance" "web" {
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
}
`)
	tmpFile := createTempFile(t, content)

	oldProvider := os.Getenv("CLOUD_PROVIDER")
	defer os.Setenv("CLOUD_PROVIDER", oldProvider)
	os.Setenv("CLOUD_PROVIDER", "aws")

	oldAccessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	oldSecretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	oldRegion := os.Getenv("AWS_REGION")
	defer func() {
		os.Setenv("AWS_ACCESS_KEY_ID", oldAccessKey)
		os.Setenv("AWS_SECRET_ACCESS_KEY", oldSecretKey)
		os.Setenv("AWS_REGION", oldRegion)
	}()
	os.Setenv("AWS_ACCESS_KEY_ID", "test-key")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")
	os.Setenv("AWS_REGION", "us-west-2")

	logger.Init(true)

	stateInstances := []cloud.Instance{
		{
			InstanceID:   "i-1234567890",
			AMI:          "ami-123456",
			InstanceType: "t2.micro",
			Tags: map[string]string{
				"Name":        "web-server",
				"Environment": "production",
			},
			RootBlockDevice: struct {
				VolumeSize int    `json:"volume_size"`
				VolumeType string `json:"volume_type"`
			}{
				VolumeSize: 20,
				VolumeType: "gp2",
			},
		},
	}

	configInstances := []cloud.Instance{
		{
			InstanceID:   "i-1234567890",
			AMI:          "ami-789012",
			InstanceType: "t2.micro",
			Tags: map[string]string{
				"Name":        "web-server",
				"Environment": "production",
			},
			RootBlockDevice: struct {
				VolumeSize int    `json:"volume_size"`
				VolumeType string `json:"volume_type"`
			}{
				VolumeSize: 20,
				VolumeType: "gp2",
			},
		},
	}

	testApp := app.NewTestable(
		env.GeneralConfig{StatePath: tmpFile},
		func(ctx context.Context, cfg config.ProviderConfig) ([]cloud.Instance, error) {
			return stateInstances, nil
		},
		func(content []byte) ([]cloud.Instance, error) {
			return configInstances, nil
		},
	)

	err := testApp.Run(context.Background(), []string{"ami"}, parser.Terraform, ports.HTTP)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "drift detected")

	var driftErr customErr.ErrDriftDetected
	assert.True(t, errors.As(err, &driftErr), "expected error to be of type ErrDriftDetected")
}

func TestRunWithNoDriftDetected(t *testing.T) {
	content := []byte(`
provider "aws" {
  region = "us-west-2"
}
resource "aws_instance" "web" {
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
}
`)
	tmpFile := createTempFile(t, content)

	oldProvider := os.Getenv("CLOUD_PROVIDER")
	defer os.Setenv("CLOUD_PROVIDER", oldProvider)
	os.Setenv("CLOUD_PROVIDER", "aws")

	oldAccessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	oldSecretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	oldRegion := os.Getenv("AWS_REGION")
	defer func() {
		os.Setenv("AWS_ACCESS_KEY_ID", oldAccessKey)
		os.Setenv("AWS_SECRET_ACCESS_KEY", oldSecretKey)
		os.Setenv("AWS_REGION", oldRegion)
	}()
	os.Setenv("AWS_ACCESS_KEY_ID", "test-key")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")
	os.Setenv("AWS_REGION", "us-west-2")

	logger.Init(true)

	instances := []cloud.Instance{
		{
			InstanceID:   "i-1234567890",
			AMI:          "ami-123456",
			InstanceType: "t2.micro",
			Tags: map[string]string{
				"Name":        "web-server",
				"Environment": "production",
			},
			RootBlockDevice: struct {
				VolumeSize int    `json:"volume_size"`
				VolumeType string `json:"volume_type"`
			}{
				VolumeSize: 20,
				VolumeType: "gp2",
			},
		},
	}

	testApp := app.NewTestable(
		env.GeneralConfig{StatePath: tmpFile},
		func(ctx context.Context, cfg config.ProviderConfig) ([]cloud.Instance, error) {
			return instances, nil
		},
		func(content []byte) ([]cloud.Instance, error) {
			return instances, nil
		},
	)

	err := testApp.Run(context.Background(), []string{"ami"}, parser.Terraform, ports.HTTP)
	assert.NoError(t, err)
}

func createTempFile(t *testing.T, content []byte) string {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "temp.tf")
	err := os.WriteFile(tmpFile, content, 0644)
	require.NoError(t, err)
	return tmpFile
}
