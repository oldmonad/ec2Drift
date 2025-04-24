package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/oldmonad/ec2Drift.git/internal/parser"
)

var Abs = filepath.Abs

type Config struct {
	OldStatePath    string
	NewStatePath    string
	Attributes      []string
	OldStateContent []byte
	NewStateContent []byte
	TerraformState  *parser.TerraformStateFile
	TerraformConfig *parser.TerraformConfig
}

// Custom error type for missing EC2 instances
type ErrNoEC2Instances struct {
	Path string
}

func (e ErrNoEC2Instances) Error() string {
	return fmt.Sprintf("no AWS EC2 instances found in state file: %s",
		filepath.Base(e.Path))
}

func NewFromFlags() (*Config, error) {
	cfg := &Config{}

	// Parse command-line flags
	flag.StringVar(&cfg.OldStatePath, "old-state", "", "Path to old Terraform state file (required)")
	flag.StringVar(&cfg.NewStatePath, "new-state", "", "Path to new Terraform configuration file (required)")
	attributes := flag.String("attributes", "", "Comma-separated list of attributes to check")
	flag.Parse()

	// Validate required parameters
	if cfg.OldStatePath == "" || cfg.NewStatePath == "" {
		return nil, fmt.Errorf("both --old-state and --new-state are required")
	}

	// Process attributes
	if *attributes != "" {
		cfg.Attributes = strings.Split(*attributes, ",")
	} else {
		cfg.Attributes = []string{
			"ami",
			"arn",
			"instance_type",
			"security_groups",
			"source_dest_check",
			"subnet_id",
			"tags",
			"vpc_security_group_ids",
		}
	}

	return cfg, nil
}

func (c *Config) LoadAndValidate() error {
	// Check file existence and readability
	if err := ValidateFile(c.OldStatePath); err != nil {
		return fmt.Errorf("old state file: %w", err)
	}

	if err := ValidateFile(c.NewStatePath); err != nil {
		return fmt.Errorf("new state file: %w", err)
	}
	// Read file contents
	var err error

	if err = c.ReadStateFiles(); err != nil {
		return fmt.Errorf("error reading state files: %w", err)
	}

	// Parse Terraform state
	state, err := parser.ParseTerraformState(c.OldStateContent)
	if err != nil {
		return fmt.Errorf("state file parse error: %w", err)
	}
	c.TerraformState = state

	// Validate EC2 instances exist
	if !state.HasEC2Instances() {
		return ErrNoEC2Instances{Path: c.OldStatePath}
	}

	// Parse TFConfig
	tfConfig, err := parser.ParseTerraformConfig(c.NewStateContent)

	if err != nil {
		return fmt.Errorf("tfconfig parse error: %w", err)
	}
	c.TerraformConfig = tfConfig

	return nil
}

func (c *Config) ReadStateFiles() error {
	var err error
	if c.OldStateContent, err = os.ReadFile(c.OldStatePath); err != nil {
		return fmt.Errorf("error reading old state file: %w", err)
	}

	if c.NewStateContent, err = os.ReadFile(c.NewStatePath); err != nil {
		return fmt.Errorf("error reading new state file: %w", err)
	}

	return nil
}

func ValidateFile(path string) error {
	absPath, err := Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", absPath)
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory: %s", absPath)
	}

	return nil
}
