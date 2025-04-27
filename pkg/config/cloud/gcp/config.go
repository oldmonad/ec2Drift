package gcp

import (
	"fmt"
	"os"
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
	if c.ProjectID == "" || c.Region == "" {
		return fmt.Errorf("missing GCP configuration")
	}
	return nil
}

func (c *Config) GetCredentials() interface{} {
	return c.CredentialsFile
}

func (c *Config) GetRegion() string {
	return c.Region
}
