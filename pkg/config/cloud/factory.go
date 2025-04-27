package cloud

import (
	"fmt"

	"github.com/oldmonad/ec2Drift/pkg/config/cloud/aws"
	"github.com/oldmonad/ec2Drift/pkg/config/cloud/gcp"
	"github.com/oldmonad/ec2Drift/pkg/logger"
	"go.uber.org/zap"
)

type ProviderConfig interface {
	Validate() error
	GetCredentials() interface{}
	GetRegion() string
}

type ProviderType string

const (
	AWS ProviderType = "aws"
	GCP ProviderType = "gcp"
)

func NewProviderConfig(provider ProviderType) (ProviderConfig, error) {
	switch provider {
	case AWS:
		cfg := aws.LoadConfig()
		logger.Log.Debug("Loaded AWS configuration",
			zap.String("access_key", cfg.AccessKey[:4]+"****"),
			zap.String("region", cfg.Region))

		if err := cfg.Validate(); err != nil {
			logger.Log.Error("AWS configuration validation failed",
				zap.Error(err))
			return nil, fmt.Errorf("aws config error: %w", err)
		}
		return cfg, nil

	// The GCP config here is just here to demonstrate the
	// extensibility of this module to support other cloud providers.
	case GCP:
		cfg := gcp.LoadConfig()
		if err := cfg.Validate(); err != nil {
			return nil, fmt.Errorf("gcp config error: %w", err)
		}
		return cfg, nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}
