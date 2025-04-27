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

func TestMain(m *testing.M) {
	logger.SetLogger(zap.NewNop())
	os.Exit(m.Run())
}

func TestTerraformParser_Parse(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    []cloud.Instance
		expectError bool
		errorMsg    string
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
			expected:    []cloud.Instance{},
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := &parser.TerraformParser{}
			instances, err := parser.Parse([]byte(tt.input))

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				assert.Nil(t, instances)
			} else {
				require.NoError(t, err)
				require.Equal(t, len(tt.expected), len(instances), "number of instances mismatch")

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
