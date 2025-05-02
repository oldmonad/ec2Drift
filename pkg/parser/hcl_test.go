package parser_test

import (
	"os"
	"testing"

	"github.com/oldmonad/ec2Drift/pkg/cloud"
	"github.com/oldmonad/ec2Drift/pkg/logger"
	"github.com/oldmonad/ec2Drift/pkg/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestMain is the entry point for the test suite.
// It sets a no-op logger to suppress logs during test execution.
func TestMain(m *testing.M) {
	logger.SetLogger(zap.NewNop())
	os.Exit(m.Run())
}

// TestTerraformParser_Parse verifies the behavior of the TerraformParser's Parse method
// under different HCL input scenarios.
func TestTerraformParser_Parse(t *testing.T) {
	// Define test cases
	tests := []struct {
		name        string           // Descriptive name of the test case
		input       string           // HCL input to be parsed
		expected    []cloud.Instance // Expected result after parsing
		expectError bool             // Whether an error is expected
		errorMsg    string           // Substring expected in the error message, if any
	}{
		{
			name: "valid HCL with two EC2 instances",
			input: `
provider "aws" {
  profile = "test-user-profile"
  region  = "eu-west-1"
}

resource "aws_instance" "web" {
  ami           = "ami-0ce8c2b29fcc8a946"
  instance_type = "t2.small"
  tags = {
    Name        = "web-server"
    Environment = "production"
  }
  root_block_device {
    volume_size = 28
    volume_type = "gp3"
  }
}

resource "aws_instance" "db" {
  ami           = "ami-0ce8c2b29fcc8a146"
  instance_type = "t3.large"
  tags = {
    Name        = "db-server"
    Environment = "production"
  }
  root_block_device {
    volume_size = 26
    volume_type = "gp4"
  }
}
`,
			expected: []cloud.Instance{
				{
					InstanceID:     "web",
					AMI:            "ami-0ce8c2b29fcc8a946",
					InstanceType:   "t2.small",
					SecurityGroups: []string{},
					Tags: map[string]string{
						"Name":        "web-server",
						"Environment": "production",
					},
					RootBlockDevice: struct {
						VolumeSize int    `json:"volume_size"`
						VolumeType string `json:"volume_type"`
					}{
						VolumeSize: 28,
						VolumeType: "gp3",
					},
				},
				{
					InstanceID:     "db",
					AMI:            "ami-0ce8c2b29fcc8a146",
					InstanceType:   "t3.large",
					SecurityGroups: []string{},
					Tags: map[string]string{
						"Name":        "db-server",
						"Environment": "production",
					},
					RootBlockDevice: struct {
						VolumeSize int    `json:"volume_size"`
						VolumeType string `json:"volume_type"`
					}{
						VolumeSize: 26,
						VolumeType: "gp4",
					},
				},
			},
			expectError: false,
		},
		{
			name: "invalid HCL syntax (parse error)",
			input: `
		resource "aws_instance" "test" {
		  ami = "ami-123"
		`,
			expected:    nil,
			expectError: true,
			errorMsg:    "failed to parse HCL",
		},
		{
			name: "non-EC2 resource",
			input: `
		resource "aws_s3_bucket" "test" {
		  bucket = "my-bucket"
		}
		`,
			expected:    []cloud.Instance{}, // No EC2 instances expected
			expectError: false,
		},
		{
			name: "minimal EC2 instance configuration",
			input: `
		resource "aws_instance" "minimal" {
		  ami           = "ami-minimal"
		  instance_type = "t2.nano"
		}
		`,
			expected: []cloud.Instance{
				{
					InstanceID:     "minimal",
					AMI:            "ami-minimal",
					InstanceType:   "t2.nano",
					SecurityGroups: []string{},
					Tags:           map[string]string{},
					RootBlockDevice: struct {
						VolumeSize int    `json:"volume_size"`
						VolumeType string `json:"volume_type"`
					}{},
				},
			},
			expectError: false,
		},
		{
			name: "fallback decoding on invalid fields",
			input: `
				resource "aws_instance" "fallback" {
				  ami           = "ami-fallback"
				  instance_type = "t2.medium"
				  invalid_field = "value"
				}
				`,
			expected: []cloud.Instance{
				{
					InstanceID:     "fallback",
					AMI:            "ami-fallback",
					InstanceType:   "t2.medium",
					SecurityGroups: []string{},
					Tags:           map[string]string{},
					RootBlockDevice: struct {
						VolumeSize int    `json:"volume_size"`
						VolumeType string `json:"volume_type"`
					}{},
				},
			},
			expectError: false,
		},
	}

	// Iterate through test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new instance of the TerraformParser
			parser := &parser.TerraformParser{}

			// Call the parser with the provided HCL input
			instances, err := parser.Parse([]byte(tt.input))

			if tt.expectError {
				// Assert that an error occurred
				require.Error(t, err)

				// If an expected error message is provided, verify it is contained in the actual error
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}

				// Assert that result is nil when an error occurs
				assert.Nil(t, instances)
			} else {
				// Assert that no error occurred
				require.NoError(t, err)

				// Ensure the number of parsed instances matches the expected number
				require.Equal(t, len(tt.expected), len(instances), "number of instances mismatch")

				// Compare each parsed instance with expected
				for i, expected := range tt.expected {
					actual := instances[i]
					assert.Equal(t, expected.InstanceID, actual.InstanceID)
					assert.Equal(t, expected.AMI, actual.AMI)
					assert.Equal(t, expected.InstanceType, actual.InstanceType)
					assert.Equal(t, expected.Tags, actual.Tags)
					assert.Equal(t, expected.SecurityGroups, actual.SecurityGroups)
					assert.Equal(t, expected.RootBlockDevice.VolumeSize, actual.RootBlockDevice.VolumeSize)
					assert.Equal(t, expected.RootBlockDevice.VolumeType, actual.RootBlockDevice.VolumeType)
				}
			}
		})
	}
}
