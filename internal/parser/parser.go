package parser

import (
	"encoding/json"
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/zclconf/go-cty/cty"
)

// State represents the structure of a Terraform state file
type TerraformStateFile struct {
	Resources []Resource `json:"resources"`
}

// Resource represents a Terraform resource in the state file
type Resource struct {
	Type      string     `json:"type"`
	Name      string     `json:"name"`
	Mode      string     `json:"mode"`
	Module    string     `json:"module,omitempty"`
	Instances []Instance `json:"instances"`
}

// Instance represents a Terraform resource instance
type Instance struct {
	IndexKey   string             `json:"index_key"`
	Attributes InstanceAttributes `json:"attributes"`
}

// InstanceAttributes contains detailed configuration of an EC2 instance
type InstanceAttributes struct {
	ID                  string            `json:"id"`
	AMI                 string            `json:"ami"`
	InstanceType        string            `json:"instance_type"`
	SubnetID            string            `json:"subnet_id"`
	Tags                map[string]string `json:"tags"`
	SourceDestCheck     bool              `json:"source_dest_check"`
	SecurityGroups      []string          `json:"security_groups"`
	VPCSecurityGroupIDs []string          `json:"vpc_security_group_ids"`
}

type InstanceConfig struct {
	ApplicationName     string   `hcl:"application_name" cty:"application_name"`
	AMI                 string   `hcl:"ami" cty:"ami"`
	InstanceType        string   `hcl:"instance_type" cty:"instance_type"`
	NoOfInstances       int      `hcl:"no_of_instances" cty:"no_of_instances"`
	VPCSecurityGroupIDs []string `hcl:"vpc_security_group_ids,optional" cty:"vpc_security_group_ids"`
	SubnetID            *string  `hcl:"subnet_id,optional" cty:"subnet_id"`
	SecurityGroups      []string `hcl:"security_groups,optional" cty:"security_groups"`
	SourceDestCheck     *bool    `hcl:"source_dest_check,optional" cty:"source_dest_check"`
}

type TerraformConfig struct {
	Configurations []InstanceConfig `hcl:"configurations" cty:"configurations"`
}

var (
	ParseTerraformState  = parseTerraformState
	ParseTerraformConfig = parseTerraformConfig
)

// ParseState parses Terraform state file content
func parseTerraformState(content []byte) (*TerraformStateFile, error) {
	var state TerraformStateFile
	if err := json.Unmarshal(content, &state); err != nil {
		return nil, fmt.Errorf("JSON parse error: %w", err)
	}
	return &state, nil
}

func parseTerraformConfig(content []byte) (*TerraformConfig, error) {
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL(content, "input.tfvars")
	if diags.HasErrors() {
		return nil, fmt.Errorf("HCL parse error: %w", diags)
	}

	// Extract the raw attribute
	var rawConfig struct {
		ConfigurationsAttr hcl.Expression `hcl:"configurations"`
	}

	ctx := &hcl.EvalContext{
		Variables: map[string]cty.Value{
			"var": cty.EmptyObjectVal,
		},
	}

	diags = gohcl.DecodeBody(file.Body, ctx, &rawConfig)
	if diags.HasErrors() {
		return nil, fmt.Errorf("HCL decode error: %w", diags)
	}

	// Evaluate the configurations expression to a cty.Value
	configurationsVal, diags := rawConfig.ConfigurationsAttr.Value(ctx)
	if diags.HasErrors() {
		return nil, fmt.Errorf("configurations eval error: %w", diags)
	}

	// Create the final config
	config := &TerraformConfig{
		Configurations: []InstanceConfig{},
	}

	// Ensure we have a list of objects
	if !configurationsVal.Type().IsTupleType() && !configurationsVal.Type().IsListType() {
		return nil, fmt.Errorf("HCL decode error: configurations must be a list, got %s", configurationsVal.Type().FriendlyName())
	}

	// Process each item in the list
	for i, it := 0, configurationsVal.ElementIterator(); it.Next(); i++ {
		_, instanceVal := it.Element()

		// Ensure the element is an object
		if !instanceVal.Type().IsObjectType() {
			return nil, fmt.Errorf("HCL decode error: each configuration must be an object")
		}

		// Create a new instance config
		instance := InstanceConfig{}

		// Check for required fields
		attrTypes := instanceVal.Type().AttributeTypes()

		// Check for application_name
		if _, hasAppName := attrTypes["application_name"]; !hasAppName {
			return nil, fmt.Errorf("HCL decode error: application_name is required for configuration %d", i)
		}
		appNameVal := instanceVal.GetAttr("application_name")
		if appNameVal.IsNull() {
			return nil, fmt.Errorf("HCL decode error: application_name cannot be null for configuration %d", i)
		}
		instance.ApplicationName = appNameVal.AsString()

		// Check for ami
		if _, hasAmi := attrTypes["ami"]; !hasAmi {
			return nil, fmt.Errorf("HCL decode error: ami is required for configuration %d", i)
		}
		amiVal := instanceVal.GetAttr("ami")
		if amiVal.IsNull() {
			return nil, fmt.Errorf("HCL decode error: ami cannot be null for configuration %d", i)
		}
		instance.AMI = amiVal.AsString()

		// Check for instance_type
		if _, hasInstanceType := attrTypes["instance_type"]; !hasInstanceType {
			return nil, fmt.Errorf("HCL decode error: instance_type is required for configuration %d", i)
		}
		instanceTypeVal := instanceVal.GetAttr("instance_type")
		if instanceTypeVal.IsNull() {
			return nil, fmt.Errorf("HCL decode error: instance_type cannot be null for configuration %d", i)
		}
		instance.InstanceType = instanceTypeVal.AsString()

		// Check for no_of_instances
		if _, hasNoOfInstances := attrTypes["no_of_instances"]; !hasNoOfInstances {
			return nil, fmt.Errorf("HCL decode error: no_of_instances is required for configuration %d", i)
		}
		noOfInstancesVal := instanceVal.GetAttr("no_of_instances")
		if noOfInstancesVal.IsNull() {
			return nil, fmt.Errorf("HCL decode error: no_of_instances cannot be null for configuration %d", i)
		}
		bf := noOfInstancesVal.AsBigFloat()
		intVal, _ := bf.Int64()
		instance.NoOfInstances = int(intVal)

		// Process optional fields
		// VpcSecurityGroupIDs
		if _, hasVpcSG := attrTypes["vpc_security_group_ids"]; hasVpcSG {
			vpcSGVal := instanceVal.GetAttr("vpc_security_group_ids")
			if !vpcSGVal.IsNull() && (vpcSGVal.Type().IsListType() || vpcSGVal.Type().IsTupleType()) {
				for it := vpcSGVal.ElementIterator(); it.Next(); {
					_, v := it.Element()
					instance.VPCSecurityGroupIDs = append(instance.VPCSecurityGroupIDs, v.AsString())
				}
			}
		}

		// SubnetID
		if _, hasSubnetID := attrTypes["subnet_id"]; hasSubnetID {
			subnetVal := instanceVal.GetAttr("subnet_id")
			if !subnetVal.IsNull() {
				subnetID := subnetVal.AsString()
				instance.SubnetID = &subnetID
			}
		}

		// SecurityGroups
		if _, hasSG := attrTypes["security_groups"]; hasSG {
			sgVal := instanceVal.GetAttr("security_groups")
			if !sgVal.IsNull() && (sgVal.Type().IsListType() || sgVal.Type().IsTupleType()) {
				for it := sgVal.ElementIterator(); it.Next(); {
					_, v := it.Element()
					instance.SecurityGroups = append(instance.SecurityGroups, v.AsString())
				}
			}
		}

		if _, ok := attrTypes["source_dest_check"]; ok {
			sdcVal := instanceVal.GetAttr("source_dest_check")
			if !sdcVal.IsNull() {
				b := sdcVal.True()
				instance.SourceDestCheck = &b
			}
		}

		config.Configurations = append(config.Configurations, instance)
	}

	return config, nil
}

// HasEC2Instances checks if the state contains AWS EC2 resources
func (s *TerraformStateFile) HasEC2Instances() bool {
	for _, resource := range s.Resources {
		if resource.Type == "aws_instance" {
			return true
		}
	}
	return false
}

// GetEC2Instances returns all EC2 instances from the state file
func (s *TerraformStateFile) GetEC2Instances() []Instance {
	var instances []Instance
	for _, resource := range s.Resources {
		if resource.Type == "aws_instance" {
			instances = append(instances, resource.Instances...)
		}
	}
	return instances
}

// GetConfigCount returns the number of configurations in the tfvars file
func (c *TerraformConfig) GetConfigCount() int {
	return len(c.Configurations)
}

func (t *TerraformConfig) GetConfigs() []*InstanceConfig {
	configs := make([]*InstanceConfig, len(t.Configurations))
	for i := range t.Configurations {
		configs[i] = &t.Configurations[i]
	}
	return configs
}
