package parser_test

import (
	"testing"

	"github.com/oldmonad/ec2Drift.git/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTerraformStatesValid(t *testing.T) {
	content := []byte(`{
		"resources": [
			{
				"type": "aws_instance",
				"name": "example",
				"mode": "managed",
				"instances": [{"attributes": {"id": "i-123"}}]
			}
		]
	}`)
	state, err := parser.ParseTerraformState(content)
	require.NoError(t, err)
	assert.True(t, state.HasEC2Instances())
	assert.Len(t, state.GetEC2Instances(), 1)
}

func TestParseTerraformStateInvalidJSON(t *testing.T) {
	content := []byte(`invalid json`)
	_, err := parser.ParseTerraformState(content)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JSON parse error")
}

func TestParseTerraformStateNoEC2Instances(t *testing.T) {
	content := []byte(`{"resources": [{"type": "aws_s3_bucket"}]}`)
	state, err := parser.ParseTerraformState(content)
	require.NoError(t, err)
	assert.False(t, state.HasEC2Instances())
}

func TestParseTerraformConfigValid(t *testing.T) {
	hclContent := []byte(`
		configurations = [{
			application_name = "app"
			ami             = "ami-123"
			instance_type   = "t2.small"
			no_of_instances = 2
			subnet_id       = "subnet-123"
			security_groups = ["sg-1"]
			source_dest_check = true
		}]
	`)
	config, err := parser.ParseTerraformConfig(hclContent)
	require.NoError(t, err)
	require.Len(t, config.Configurations, 1)
	assert.Equal(t, "app", config.Configurations[0].ApplicationName)
	assert.Equal(t, "ami-123", config.Configurations[0].AMI)
	assert.Equal(t, 2, config.Configurations[0].NoOfInstances)
	assert.Equal(t, "subnet-123", *config.Configurations[0].SubnetID)
	assert.Equal(t, []string{"sg-1"}, config.Configurations[0].SecurityGroups)
	assert.True(t, *config.Configurations[0].SourceDestCheck)
}

func TestParseTerraformConfigMissingRequiredFields(t *testing.T) {
	hclContent := []byte(`
		configurations = [{
			ami             = "ami-123"
			instance_type   = "t2.micro"
			no_of_instances = 1
		}]
	`)
	_, err := parser.ParseTerraformConfig(hclContent)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "application_name is required")
}

func TestParseTerraformConfigInvalidHCLSyntax(t *testing.T) {
	hclContent := []byte(`configurations = [ invalid `)
	_, err := parser.ParseTerraformConfig(hclContent)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HCL parse error")
}

func TestParseTerraformConfigOptionalFieldsNull(t *testing.T) {
	hclContent := []byte(`
		configurations = [{
			application_name = "app"
			ami             = "ami-123"
			instance_type   = "t2.micro"
			no_of_instances = 1
			subnet_id       = null
		}]
	`)
	config, err := parser.ParseTerraformConfig(hclContent)
	require.NoError(t, err)
	assert.Nil(t, config.Configurations[0].SubnetID)
}

func TestParseTerraformConfigEmptyConfigurations(t *testing.T) {
	hclContent := []byte(`configurations = []`)
	config, err := parser.ParseTerraformConfig(hclContent)
	require.NoError(t, err)
	assert.Equal(t, 0, config.GetConfigCount())
}

func TestParseTerraformConfigNoOfInstancesFloat(t *testing.T) {
	hclContent := []byte(`
		configurations = [{
			application_name = "app"
			ami             = "ami-123"
			instance_type   = "t2.micro"
			no_of_instances = 2.5
		}]
	`)
	config, err := parser.ParseTerraformConfig(hclContent)
	require.NoError(t, err)
	assert.Equal(t, 2, config.Configurations[0].NoOfInstances)
}

func TestParseTerraformConfigWrongTypeOptionalField(t *testing.T) {
	hclContent := []byte(`
		configurations = [{
			application_name = "app"
			ami             = "ami-123"
			instance_type   = "t2.micro"
			no_of_instances = 1
			vpc_security_group_ids = "sg-123"
		}]
	`)
	config, err := parser.ParseTerraformConfig(hclContent)
	require.NoError(t, err)
	assert.Empty(t, config.Configurations[0].VPCSecurityGroupIDs)
}

func TestTerraformStateFileGetEC2Instances(t *testing.T) {
	state := &parser.TerraformStateFile{
		Resources: []parser.Resource{
			{Type: "aws_instance", Instances: []parser.Instance{{IndexKey: "0"}}},
		},
	}

	instances := state.GetEC2Instances()
	assert.Len(t, instances, 1)
}

func TestTerraformConfigGetConfigs(t *testing.T) {
	config := &parser.TerraformConfig{
		Configurations: []parser.InstanceConfig{{}, {}},
	}
	configs := config.GetConfigs()
	assert.Len(t, configs, 2)
	assert.IsType(t, &parser.InstanceConfig{}, configs[0])
}

func TestParseTerraformConfigConfigurationsNotList(t *testing.T) {
	hclContent := []byte(`configurations = {}`)
	_, err := parser.ParseTerraformConfig(hclContent)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be a list")
}

func TestParseTerraformConfigInvalidInstanceType(t *testing.T) {
	hclContent := []byte(`
		configurations = [ "invalid" ]
	`)
	_, err := parser.ParseTerraformConfig(hclContent)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be an object")
}

func TestParseTerraformConfigMissingConfigurationsAttribute(t *testing.T) {
	hclContent := []byte(`some_attr = "value"`)
	_, err := parser.ParseTerraformConfig(hclContent)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HCL decode error")
}

func TestParseTerraformConfigExtraAttributes(t *testing.T) {
	hclContent := []byte(`
        configurations = []
        extra_attr = "value"
    `)
	_, err := parser.ParseTerraformConfig(hclContent)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HCL decode error")
}

func TestParseTerraformConfigUndefinedVariable(t *testing.T) {
	hclContent := []byte(`configurations = var.undefined_var`)
	_, err := parser.ParseTerraformConfig(hclContent)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "configurations eval error")
}

func TestParseTerraformConfigInvalidExpression(t *testing.T) {
	hclContent := []byte(`configurations = "invalid"`)
	_, err := parser.ParseTerraformConfig(hclContent)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be a list")
}

func TestParseTerraformConfigApplicationNameRequired(t *testing.T) {
	hclContent := []byte(`
        configurations = [{
            ami = "ami-123"
            instance_type = "t2.micro"
            no_of_instances = 1
        }]
    `)
	_, err := parser.ParseTerraformConfig(hclContent)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "application_name is required")
}

func TestParseTerraformConfigApplicationNameNull(t *testing.T) {
	hclContent := []byte(`
        configurations = [{
            application_name = null
            ami = "ami-123"
            instance_type = "t2.micro"
            no_of_instances = 1
        }]
    `)
	_, err := parser.ParseTerraformConfig(hclContent)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "application_name cannot be null")
}

func TestParseTerraformConfigAMIRequired(t *testing.T) {
	hclContent := []byte(`
        configurations = [{
            application_name = "app"
            instance_type = "t2.micro"
            no_of_instances = 1
        }]
    `)
	_, err := parser.ParseTerraformConfig(hclContent)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ami is required")
}

func TestParseTerraformConfigAMINull(t *testing.T) {
	hclContent := []byte(`
        configurations = [{
            application_name = "app"
            ami = null
            instance_type = "t2.micro"
            no_of_instances = 1
        }]
    `)
	_, err := parser.ParseTerraformConfig(hclContent)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ami cannot be null")
}

func TestParseTerraformConfigInstanceTypeRequired(t *testing.T) {
	hclContent := []byte(`
        configurations = [{
            application_name = "app"
            ami = "ami-123"
            no_of_instances = 1
        }]
    `)
	_, err := parser.ParseTerraformConfig(hclContent)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "instance_type is required")
}

func TestParseTerraformConfigInstanceTypeNull(t *testing.T) {
	hclContent := []byte(`
        configurations = [{
            application_name = "app"
            ami = "ami-123"
            instance_type = null
            no_of_instances = 1
        }]
    `)
	_, err := parser.ParseTerraformConfig(hclContent)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "instance_type cannot be null")
}

func TestParseTerraformConfigNoOfInstancesRequired(t *testing.T) {
	hclContent := []byte(`
        configurations = [{
            application_name = "app"
            ami = "ami-123"
            instance_type = "t2.micro"
        }]
    `)
	_, err := parser.ParseTerraformConfig(hclContent)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no_of_instances is required")
}

func TestParseTerraformConfigNoOfInstancesNull(t *testing.T) {
	hclContent := []byte(`
        configurations = [{
            application_name = "app"
            ami = "ami-123"
            instance_type = "t2.micro"
            no_of_instances = null
        }]
    `)
	_, err := parser.ParseTerraformConfig(hclContent)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no_of_instances cannot be null")
}

func TestParseTerraformConfigVPCSecurityGroupIDsProcessing(t *testing.T) {
	hclContent := []byte(`
        configurations = [{
            application_name = "app"
            ami = "ami-123"
            instance_type = "t2.micro"
            no_of_instances = 1
            vpc_security_group_ids = ["sg-1", "sg-2"]
        }]
    `)
	config, err := parser.ParseTerraformConfig(hclContent)
	require.NoError(t, err)
	require.Len(t, config.Configurations, 1)
	assert.Equal(t, []string{"sg-1", "sg-2"}, config.Configurations[0].VPCSecurityGroupIDs)
}
