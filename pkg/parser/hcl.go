package parser

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/oldmonad/ec2Drift/pkg/cloud"
	"github.com/oldmonad/ec2Drift/pkg/errors"
	"github.com/oldmonad/ec2Drift/pkg/logger"
	"go.uber.org/zap"
)

// TerraformParser is a parser for Terraform HCL files
type TerraformParser struct{}

// Config represents the top-level structure of a Terraform configuration
type Config struct {
	Providers []struct {
		Name string   `hcl:"name,label"`  // e.g. aws
		Body hcl.Body `hcl:",remain"`     // raw body for future extensions
	} `hcl:"provider,block"`
	Resources []Resource `hcl:"resource,block"` // All defined resources in the file
}

// Resource holds the type, name, and body of a Terraform resource block
type Resource struct {
	Type string   `hcl:"type,label"` // e.g. aws_instance
	Name string   `hcl:"name,label"` // resource name identifier
	Body hcl.Body `hcl:",remain"`    // raw body of the resource block
}

// EC2Instance models attributes specific to aws_instance
type EC2Instance struct {
	AMI             string            `hcl:"ami"`                        // AMI ID
	InstanceType    string            `hcl:"instance_type"`              // EC2 instance type
	Tags            map[string]string `hcl:"tags,optional"`              // Optional tags
	RootBlockDevice *RootBlockDevice  `hcl:"root_block_device,block"`    // Optional root block device config
}

// RootBlockDevice holds volume configuration for EC2 instances
type RootBlockDevice struct {
	VolumeSize int    `hcl:"volume_size,optional"` // in GiB
	VolumeType string `hcl:"volume_type,optional"` // e.g. gp2, io1
}

// Parse decodes the Terraform HCL content and extracts EC2 instances
func (p *TerraformParser) Parse(content []byte) ([]cloud.Instance, error) {
	config, err := parseTerraformFile(content)
	if err != nil {
		return nil, err
	}

	instances, err := config.GetEC2Instances()
	if err != nil {
		return nil, err
	}

	return instances, nil
}

// parseTerraformFile parses raw HCL and populates the Config struct
func parseTerraformFile(content []byte) (*Config, error) {
	log := logger.WithField("component", "terraform-parser")
	log.Debug("Parsing Terraform file")

	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL([]byte(content), "main.tf")
	if diags.HasErrors() {
		log.Error("HCL parsing failed",
			zap.String("error", diags.Error()),
			zap.Int("error_count", len(diags)))
		for _, diag := range diags {
			log.Debug("Parsing diagnostic",
				zap.String("summary", diag.Summary),
				zap.String("detail", diag.Detail),
				zap.String("position", fmt.Sprintf("%v", diag.Subject)))
		}
		return nil, errors.ErrHCLParseFailure{Diagnostics: diags}
	}

	var config Config
	diags = gohcl.DecodeBody(file.Body, nil, &config)
	if diags.HasErrors() {
		log.Error("HCL decoding failed",
			zap.String("error", diags.Error()),
			zap.Int("error_count", len(diags)))
		for _, diag := range diags {
			log.Debug("Decoding diagnostic",
				zap.String("summary", diag.Summary),
				zap.String("detail", diag.Detail),
				zap.String("position", fmt.Sprintf("%v", diag.Subject)))
		}
		return nil, errors.ErrHCLDecodeFailure{Diagnostics: diags}
	}

	log.Info("Successfully parsed Terraform file",
		zap.Int("resource_count", len(config.Resources)))
	return &config, nil
}

// GetEC2Instances extracts aws_instance resources and maps them to cloud.Instance
func (config *Config) GetEC2Instances() ([]cloud.Instance, error) {
	log := logger.WithField("component", "terraform-parser")
	log.Debug("Extracting EC2 instances from Terraform config")

	var tfInstances []cloud.Instance
	for _, res := range config.Resources {
		if res.Type != "aws_instance" {
			continue
		}

		log.Debug("Found aws_instance resource", zap.String("name", res.Name))
		var instance EC2Instance
		diags := gohcl.DecodeBody(res.Body, nil, &instance)

		if diags.HasErrors() {
			log.Error("Failed to decode aws_instance resource",
				zap.String("name", res.Name),
				zap.String("error", diags.Error()))

			// Fallback: check if 'tags' exists but is not a map
			var tagsCheck struct {
				Tags interface{} `hcl:"tags,optional"`
			}
			tagsDiags := gohcl.DecodeBody(res.Body, nil, &tagsCheck)

			if !tagsDiags.HasErrors() && tagsCheck.Tags != nil {
				_, isMap := tagsCheck.Tags.(map[string]interface{})
				if !isMap {
					log.Error("Invalid tags type in aws_instance resource",
						zap.String("name", res.Name))
					return nil, errors.ErrInvalidTagsType{ResourceName: res.Name}
				}
			}

			// Fallback: decode only essential fields
			var minInst struct {
				AMI          string   `hcl:"ami"`
				InstanceType string   `hcl:"instance_type"`
				Remain       hcl.Body `hcl:",remain"`
			}
			fbDiags := gohcl.DecodeBody(res.Body, nil, &minInst)
			if fbDiags.HasErrors() {
				log.Error("Fallback decoding failed",
					zap.String("name", res.Name),
					zap.String("error", fbDiags.Error()))
				continue
			}

			instance.AMI = minInst.AMI
			instance.InstanceType = minInst.InstanceType
			log.Debug("Used fallback minimal decoding",
				zap.String("name", res.Name),
				zap.String("ami", instance.AMI),
				zap.String("instance_type", instance.InstanceType))
		}

		// Ensure tags map is not nil
		if instance.Tags == nil {
			instance.Tags = make(map[string]string)
		}

		// Map Terraform instance data to internal cloud.Instance struct
		ci := cloud.Instance{
			InstanceID:     res.Name,
			AMI:            instance.AMI,
			InstanceType:   instance.InstanceType,
			SecurityGroups: []string{},
			Tags:           instance.Tags,
		}

		// Attach root block device config if present
		if instance.RootBlockDevice != nil {
			ci.RootBlockDevice = struct {
				VolumeSize int    `json:"volume_size"`
				VolumeType string `json:"volume_type"`
			}{
				VolumeSize: instance.RootBlockDevice.VolumeSize,
				VolumeType: instance.RootBlockDevice.VolumeType,
			}
		}

		tfInstances = append(tfInstances, ci)
	}

	log.Info("Extracted EC2 instances from Terraform config",
		zap.Int("count", len(tfInstances)))
	return tfInstances, nil
}
