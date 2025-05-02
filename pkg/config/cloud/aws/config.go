package aws

import (
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/oldmonad/ec2Drift/pkg/errors"
	"github.com/oldmonad/ec2Drift/pkg/logger"
	"go.uber.org/zap"
)

type Config struct {
	AccessKey    string
	SecretKey    string
	Region       string
	SessionToken string
}

func LoadConfig() *Config {
	return &Config{
		AccessKey:    os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretKey:    os.Getenv("AWS_SECRET_ACCESS_KEY"),
		Region:       os.Getenv("AWS_REGION"),
		SessionToken: os.Getenv("AWS_SESSION_TOKEN"),
	}
}

func (c *Config) Validate() error {
	var missing []string
	if c.AccessKey == "" {
		missing = append(missing, "AWS_ACCESS_KEY_ID")
	}
	if c.SecretKey == "" {
		missing = append(missing, "AWS_SECRET_ACCESS_KEY")
	}
	if c.Region == "" {
		missing = append(missing, "AWS_REGION")
	}

	if c.SessionToken == "" {
		missing = append(missing, "AWS_SESSION_TOKEN")
	}

	if len(missing) > 0 {
		logger.Log.Error("AWS config validation failed", zap.Strings("missing", missing))
		return errors.NewErrMissingCredentials(missing)
	}
	return nil
}

func (c *Config) GetCredentials() interface{} {
	return aws.Credentials{
		AccessKeyID:     c.AccessKey,
		SecretAccessKey: c.SecretKey,
		SessionToken:    c.SessionToken,
	}
}

func (c *Config) GetRegion() string {
	return c.Region
}
