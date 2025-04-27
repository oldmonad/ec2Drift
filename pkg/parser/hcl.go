package parser

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/oldmonad/ec2Drift/pkg/cloud"
	"github.com/oldmonad/ec2Drift/pkg/logger"
	"go.uber.org/zap"
)

type TerraformParser struct{}

type Config struct {
	Providers []struct {
		Name string   `hcl:"name,label"`
		Body hcl.Body `hcl:",remain"`
	} `hcl:"provider,block"`
	Resources []Resource `hcl:"resource,block"`
}

type Resource struct {
	Type string   `hcl:"type,label"`
	Name string   `hcl:"name,label"`
	Body hcl.Body `hcl:",remain"`
}

type EC2Instance struct {
	AMI             string            `hcl:"ami"`
	InstanceType    string            `hcl:"instance_type"`
	Tags            map[string]string `hcl:"tags,optional"`
	RootBlockDevice *RootBlockDevice  `hcl:"root_block_device,block"`
}

type RootBlockDevice struct {
	VolumeSize int    `hcl:"volume_size,optional"`
	VolumeType string `hcl:"volume_type,optional"`
}

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
		return nil, fmt.Errorf("failed to parse HCL: %s", diags.Error())
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
		return nil, fmt.Errorf("failed to decode HCL: %s", diags.Error())
	}

	log.Info("Successfully parsed Terraform file",
		zap.Int("resource_count", len(config.Resources)))
	return &config, nil
}

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

			var tagsCheck struct {
				Tags interface{} `hcl:"tags,optional"`
			}
			tagsDiags := gohcl.DecodeBody(res.Body, nil, &tagsCheck)

			if !tagsDiags.HasErrors() && tagsCheck.Tags != nil {
				_, isMap := tagsCheck.Tags.(map[string]interface{})
				if !isMap {
					log.Error("Invalid tags type in aws_instance resource",
						zap.String("name", res.Name))
					return nil, fmt.Errorf("failed to decode HCL: tags field must be a map")
				}
			}

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

		if instance.Tags == nil {
			instance.Tags = make(map[string]string)
		}

		ci := cloud.Instance{
			InstanceID:     res.Name,
			AMI:            instance.AMI,
			InstanceType:   instance.InstanceType,
			SecurityGroups: []string{},
			Tags:           instance.Tags,
		}
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
