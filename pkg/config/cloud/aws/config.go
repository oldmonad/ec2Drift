package aws

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/oldmonad/ec2Drift/pkg/logger"
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
	if c.AccessKey == "" || c.SecretKey == "" || c.Region == "" {
		errMsg := "Missing AWS credentials: AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, or AWS_REGION"
		logger.Log.Error(errMsg)
		return fmt.Errorf(errMsg)
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
