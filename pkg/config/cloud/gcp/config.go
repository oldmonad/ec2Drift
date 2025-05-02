package gcp

import (
	"os"

	"github.com/oldmonad/ec2Drift/pkg/errors"
)

type Config struct {
	ProjectID       string
	Region          string
	CredentialsFile string
}

func LoadConfig() *Config {
	return &Config{
		ProjectID:       os.Getenv("GCP_PROJECT"),
		Region:          os.Getenv("GCP_REGION"),
		CredentialsFile: os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"),
	}
}

func (c *Config) Validate() error {
	var missing []string
	if c.ProjectID == "" {
		missing = append(missing, "GCP_PROJECT")
	}
	if c.Region == "" {
		missing = append(missing, "GCP_REGION")
	}
	if c.CredentialsFile == "" {
		missing = append(missing, "GOOGLE_APPLICATION_CREDENTIALS")
	}
	if len(missing) > 0 {
		return errors.NewErrMissingGCPConfig(missing)
	}
	return nil
}

func (c *Config) GetCredentials() interface{} {
	return c.CredentialsFile
}

func (c *Config) GetRegion() string {
	return c.Region
}
