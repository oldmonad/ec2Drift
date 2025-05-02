package env

import (
	"os"
	"strconv"

	"github.com/oldmonad/ec2Drift/pkg/config/cloud"
	"github.com/oldmonad/ec2Drift/pkg/errors"
	"github.com/oldmonad/ec2Drift/pkg/logger"
	"go.uber.org/zap"
)

type Config interface {
	PortToString() string
	InitiateLogger()
}

type Configurations struct {
	DebugMode         bool
	LogLevel          string
	ConfigPath        string
	StatePath         string
	OutputPath        string
	CloudProviderType cloud.ProviderType
	HttpPort          int
	CloudConfig       cloud.ProviderConfig
	CloudProvider     CloudConfigProvider
}

type CloudConfigProvider interface {
	NewProviderConfig(cloud.ProviderType) (cloud.ProviderConfig, error)
}

// Add default implementation
type DefaultCloudProvider struct{}

func (d *DefaultCloudProvider) NewProviderConfig(p cloud.ProviderType) (cloud.ProviderConfig, error) {
	return cloud.NewProviderConfig(p)
}

func NewConfiguration() *Configurations {
	return &Configurations{
		// Initialize with default port
		// Can still be overridden by setting environment variable
		HttpPort:      8080,
		CloudProvider: &DefaultCloudProvider{},
	}
}

func (c *Configurations) LoadGeneralConfig() error {
	rawDebug := os.Getenv("DEBUG")
	mode, err := strconv.ParseBool(rawDebug)
	if err != nil {
		logger.Log.Error("failed to set up configuration", zap.Error(err))
		logger.Log.Info("Ensure the that DEBUG is set to true or false")
		return errors.NewErrDebugParse(rawDebug, err)
	}

	c.DebugMode = mode
	c.LogLevel = os.Getenv("LOG_LEVEL")
	c.ConfigPath = os.Getenv("CONFIG_PATH")
	c.StatePath = os.Getenv("STATE_PATH")
	c.OutputPath = os.Getenv("OUTPUT_PATH")

	if err := c.ValidateAndSetPort(); err != nil {
		logger.Log.Error("Invalid port configuration", zap.Error(err))
		logger.Log.Info("Ensure the that DEBUG is set to true or false")
		return err
	}

	provider := os.Getenv("CLOUD_PROVIDER")
	if provider == "" {
		logger.Log.Error("failed to set up configuration", zap.Error(err))
		logger.Log.Info("Ensure the that CLOUD_PROVIDER is set e.g aws, azure, gcp")
		return errors.NewErrMissingCloudProvider()
	}

	c.CloudProviderType = cloud.ProviderType(provider)

	return nil
}

func (c *Configurations) LoadCloudConfig() error {
	// Delegate to cloud package to create provider-specific config
	cloudCfg, err := c.CloudProvider.NewProviderConfig(c.CloudProviderType)
	if err != nil {
		return err
	}
	c.CloudConfig = cloudCfg
	return nil
}

func (c *Configurations) ValidateGeneralConfig() error {
	// Validate core configuration
	if c.StatePath == "" {
		return errors.NewErrMissingPaths()
	}

	// Validate cloud configuration
	if c.CloudConfig == nil {
		return errors.NewErrCloudConfigNotInit()
	}

	return c.CloudConfig.Validate()
}

func (c *Configurations) ValidateAndSetPort() error {
	portStr := os.Getenv("HTTP_PORT")
	if portStr == "" {
		return nil // Use default port (already set in constructor)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return errors.NewErrPortParse(portStr, err)
	}

	if port < 1 || port > 65535 {
		return errors.NewErrPortOutOfRange(port)
	}

	c.HttpPort = port
	return nil
}

func (c *Configurations) PortToString() string {
	return strconv.Itoa(c.HttpPort)
}

func (c *Configurations) InitiateLogger() {
	logger.Init(c.DebugMode)
}

func SetupConfigurations() (*Configurations, error) {
	configurations := NewConfiguration()

	if err := configurations.LoadGeneralConfig(); err != nil {
		return nil, err
	}

	configurations.InitiateLogger()

	if err := configurations.LoadCloudConfig(); err != nil {
		return nil, err
	}

	if err := configurations.ValidateGeneralConfig(); err != nil {
		return nil, err
	}

	return configurations, nil
}
