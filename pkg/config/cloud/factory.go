package cloud

import (
	"github.com/oldmonad/ec2Drift/pkg/config/cloud/aws"
	"github.com/oldmonad/ec2Drift/pkg/config/cloud/gcp"

	"github.com/oldmonad/ec2Drift/pkg/errors"
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
		// Check for access key validity,
		// this is not a proper validation for an access key
		// It's just to make sure the logger functions properly.
		if len(cfg.AccessKey) < 10 {
			logger.Log.Info("Ensure the that correct AWS_ACCESS_KEY_ID is set or not empty")
			return nil, errors.NewInvalidConfigCredential("Invalid AWS_ACCESS_KEY_ID")
		}

		logger.Log.Debug("Loaded AWS configuration",
			zap.String("access_key", cfg.AccessKey[:4]+"****"),
			zap.String("region", cfg.Region))

		if err := cfg.Validate(); err != nil {
			logger.Log.Error("AWS configuration validation failed",
				zap.Error(err))
			return nil, err
		}
		return cfg, nil

	// The GCP config here is just here to demonstrate the
	// extensibility of this module to support other cloud providers.
	// Hence no logging is required.
	case GCP:
		cfg := gcp.LoadConfig()
		if err := cfg.Validate(); err != nil {
			return nil, err
		}
		return cfg, nil
	default:
		return nil, errors.NewUnsupportedProvider(string(provider))
	}
}
