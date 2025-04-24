package comparator_test

import (
	"testing"

	"github.com/oldmonad/ec2Drift.git/internal/comparator"
	"github.com/oldmonad/ec2Drift.git/internal/parser"
	"github.com/stretchr/testify/assert"
)

func TestDetectDriftBasicDrift(t *testing.T) {
	stateInstances := []parser.Instance{{
		IndexKey: "app1-abc123",
		Attributes: parser.InstanceAttributes{
			ID:           "i-123",
			InstanceType: "t2.micro",
			AMI:          "ami-111",
		},
	}}

	configs := []*parser.InstanceConfig{{
		ApplicationName: "app1",
		InstanceType:    "t2.large",
		AMI:             "ami-222",
	}}

	attributes := []string{"instance_type", "ami"}

	expected := []comparator.DriftReport{{
		InstanceID:      "i-123",
		ApplicationName: "app1",
		Drifts: []comparator.DriftDetail{
			{Attribute: "instance_type", ExpectedValue: "t2.large", ActualValue: "t2.micro"},
			{Attribute: "ami", ExpectedValue: "ami-222", ActualValue: "ami-111"},
		},
	}}

	reports := comparator.DetectDrift(stateInstances, configs, attributes)
	assert.Equal(t, expected, reports)
}

func TestDetectDriftNoDrift(t *testing.T) {
	stateInstances := []parser.Instance{{
		IndexKey: "app1-xyz",
		Attributes: parser.InstanceAttributes{
			ID:           "i-456",
			InstanceType: "t2.medium",
			AMI:          "ami-333",
		},
	}}

	configs := []*parser.InstanceConfig{{
		ApplicationName: "app1",
		InstanceType:    "t2.medium",
		AMI:             "ami-333",
	}}

	attributes := []string{"instance_type", "ami"}

	reports := comparator.DetectDrift(stateInstances, configs, attributes)
	assert.Empty(t, reports)
}

func TestDetectDriftMissingConfig(t *testing.T) {
	stateInstances := []parser.Instance{{
		IndexKey: "app2-123",
		Attributes: parser.InstanceAttributes{
			ID:           "i-789",
			InstanceType: "t2.nano",
		},
	}}

	configs := []*parser.InstanceConfig{{
		ApplicationName: "app3",
		InstanceType:    "t2.xlarge",
	}}

	attributes := []string{"instance_type"}

	reports := comparator.DetectDrift(stateInstances, configs, attributes)
	assert.Empty(t, reports)
}

func TestDetectDriftUnsupportedAttributesOmitted(t *testing.T) {
	stateInstances := []parser.Instance{{
		IndexKey: "current-app-v1",
		Attributes: parser.InstanceAttributes{
			ID:           "i-123current",
			InstanceType: "t3.medium",
			AMI:          "ami-123",
		},
	}}

	configs := []*parser.InstanceConfig{{
		ApplicationName: "current-app",
		InstanceType:    "t3.large",
		AMI:             "ami-456",
	}}

	attributes := []string{
		"instance_type",
		"ami",
		"vpc_security_group_ids",
		"subnet_id",
	}

	reports := comparator.DetectDrift(stateInstances, configs, attributes)

	expected := []comparator.DriftReport{{
		InstanceID:      "i-123current",
		ApplicationName: "current-app",
		Drifts: []comparator.DriftDetail{
			{
				Attribute:     "instance_type",
				ExpectedValue: "t3.large",
				ActualValue:   "t3.medium",
			},
			{
				Attribute:     "ami",
				ExpectedValue: "ami-456",
				ActualValue:   "ami-123",
			},
		},
	}}

	assert.Equal(t, expected, reports,
		"Should handle supported fields normally while ignoring unspecified unsupported fields. "+
			"Unsupported fields not present in structs should be effectively ignored")
}

func TestDetectDriftEmptyAttributes(t *testing.T) {
	stateInstances := []parser.Instance{{
		IndexKey: "app5-xxx",
		Attributes: parser.InstanceAttributes{
			ID:           "i-555",
			InstanceType: "t2.small",
		},
	}}

	configs := []*parser.InstanceConfig{{
		ApplicationName: "app5",
		InstanceType:    "t2.large",
	}}

	attributes := []string{}

	reports := comparator.DetectDrift(stateInstances, configs, attributes)
	assert.Empty(t, reports)
}

func TestDetectDriftExtractBaseNameNoHyphen(t *testing.T) {
	stateInstances := []parser.Instance{{
		IndexKey: "app6",
		Attributes: parser.InstanceAttributes{
			ID:           "i-666",
			InstanceType: "t2.medium",
		},
	}}

	configs := []*parser.InstanceConfig{{
		ApplicationName: "app6",
		InstanceType:    "t2.xlarge",
	}}

	attributes := []string{"instance_type"}

	expected := []comparator.DriftReport{{
		InstanceID:      "i-666",
		ApplicationName: "app6",
		Drifts: []comparator.DriftDetail{
			{Attribute: "instance_type", ExpectedValue: "t2.xlarge", ActualValue: "t2.medium"},
		},
	}}

	reports := comparator.DetectDrift(stateInstances, configs, attributes)
	assert.Equal(t, expected, reports)
}

func TestDetectDriftConcurrentProcessing(t *testing.T) {
	stateInstances := []parser.Instance{
		{
			IndexKey: "app7-aaa",
			Attributes: parser.InstanceAttributes{
				ID:           "i-777",
				InstanceType: "t2.micro",
			},
		},
		{
			IndexKey: "app8-bbb",
			Attributes: parser.InstanceAttributes{
				ID:           "i-888",
				InstanceType: "t2.nano",
			},
		},
	}

	configs := []*parser.InstanceConfig{
		{
			ApplicationName: "app7",
			InstanceType:    "t2.large",
		},
		{
			ApplicationName: "app8",
			InstanceType:    "t2.medium",
		},
	}

	attributes := []string{"instance_type"}

	expected := []comparator.DriftReport{
		{
			InstanceID:      "i-777",
			ApplicationName: "app7",
			Drifts: []comparator.DriftDetail{
				{Attribute: "instance_type", ExpectedValue: "t2.large", ActualValue: "t2.micro"},
			},
		},
		{
			InstanceID:      "i-888",
			ApplicationName: "app8",
			Drifts: []comparator.DriftDetail{
				{Attribute: "instance_type", ExpectedValue: "t2.medium", ActualValue: "t2.nano"},
			},
		},
	}

	reports := comparator.DetectDrift(stateInstances, configs, attributes)
	assert.ElementsMatch(t, expected, reports)
}

func TestDetectDriftEmptyStateInstances(t *testing.T) {
	var stateInstances []parser.Instance
	configs := []*parser.InstanceConfig{{
		ApplicationName: "app9",
		InstanceType:    "t2.micro",
	}}
	attributes := []string{"instance_type"}

	reports := comparator.DetectDrift(stateInstances, configs, attributes)
	assert.Empty(t, reports)
}

func TestDetectDriftEmptyConfigs(t *testing.T) {
	stateInstances := []parser.Instance{{
		IndexKey: "app10-ccc",
		Attributes: parser.InstanceAttributes{
			ID:           "i-999",
			InstanceType: "t2.micro",
		},
	}}
	var configs []*parser.InstanceConfig
	attributes := []string{"instance_type"}

	reports := comparator.DetectDrift(stateInstances, configs, attributes)
	assert.Empty(t, reports)
}

func TestDetectDrift_SkippedAttributes(t *testing.T) {
	stateInstances := []parser.Instance{{
		IndexKey: "app11-ddd",
		Attributes: parser.InstanceAttributes{
			ID: "i-101",
		},
	}}

	configs := []*parser.InstanceConfig{{
		ApplicationName: "app11",
		NoOfInstances:   2,
	}}

	attributes := []string{"application_name", "no_of_instances"}

	reports := comparator.DetectDrift(stateInstances, configs, attributes)
	assert.Empty(t, reports, "Skipped attributes should not generate drifts")
}

func TestDetectDriftUnhandledAttribute(t *testing.T) {
	stateInstances := []parser.Instance{{
		IndexKey: "app12-eee",
		Attributes: parser.InstanceAttributes{
			ID:             "i-102",
			SecurityGroups: []string{"sg-123"},
		},
	}}

	configs := []*parser.InstanceConfig{{
		ApplicationName: "app12",
	}}

	attributes := []string{"security_groups"}

	reports := comparator.DetectDrift(stateInstances, configs, attributes)
	assert.Empty(t, reports, "security_groups is nested but not handled, so no drift detected")
}

func TestDetectDriftMissingConfigField(t *testing.T) {
	stateInstances := []parser.Instance{{
		IndexKey: "app13-fff",
		Attributes: parser.InstanceAttributes{
			ID:           "i-103",
			InstanceType: "t2.micro",
		},
	}}

	configs := []*parser.InstanceConfig{{
		ApplicationName: "app13",
	}}

	attributes := []string{"instance_type"}

	expected := []comparator.DriftReport{{
		InstanceID:      "i-103",
		ApplicationName: "app13",
		Drifts: []comparator.DriftDetail{
			{Attribute: "instance_type", ExpectedValue: "", ActualValue: "t2.micro"},
		},
	}}

	reports := comparator.DetectDrift(stateInstances, configs, attributes)
	assert.Equal(t, expected, reports)
}

func TestDetectDriftSourceDestCheck(t *testing.T) {
	t.Run("SourceDestCheck Drift", func(t *testing.T) {
		sourceDestCheck := false
		stateInstances := []parser.Instance{{
			IndexKey: "app-sdc-123",
			Attributes: parser.InstanceAttributes{
				ID:              "i-12345",
				SourceDestCheck: true,
			},
		}}

		configs := []*parser.InstanceConfig{{
			ApplicationName: "app-sdc",
			SourceDestCheck: &sourceDestCheck,
		}}

		attributes := []string{"source_dest_check"}

		expected := []comparator.DriftReport{{
			InstanceID:      "i-12345",
			ApplicationName: "app-sdc",
			Drifts: []comparator.DriftDetail{
				{Attribute: "source_dest_check", ExpectedValue: false, ActualValue: true},
			},
		}}

		reports := comparator.DetectDrift(stateInstances, configs, attributes)
		assert.Equal(t, expected, reports)
	})

	t.Run("SourceDestCheck Config Nil", func(t *testing.T) {
		stateInstances := []parser.Instance{{
			IndexKey: "app-sdc-nil",
			Attributes: parser.InstanceAttributes{
				ID:              "i-67890",
				SourceDestCheck: true,
			},
		}}

		configs := []*parser.InstanceConfig{{
			ApplicationName: "app-sdc-nil",
			// SourceDestCheck is nil
		}}

		attributes := []string{"source_dest_check"}

		reports := comparator.DetectDrift(stateInstances, configs, attributes)
		assert.Empty(t, reports)
	})
}

func TestDetectDriftSubnetID(t *testing.T) {
	subnetID := "subnet-expected"
	stateInstances := []parser.Instance{{
		IndexKey: "app-subnet-123",
		Attributes: parser.InstanceAttributes{
			ID:       "i-subnet-1",
			SubnetID: "subnet-actual",
		},
	}}

	configs := []*parser.InstanceConfig{{
		ApplicationName: "app-subnet",
		SubnetID:        &subnetID,
	}}

	attributes := []string{"subnet_id"}

	expected := []comparator.DriftReport{{
		InstanceID:      "i-subnet-1",
		ApplicationName: "app-subnet",
		Drifts: []comparator.DriftDetail{
			{Attribute: "subnet_id", ExpectedValue: "subnet-expected", ActualValue: "subnet-actual"},
		},
	}}

	reports := comparator.DetectDrift(stateInstances, configs, attributes)
	assert.Equal(t, expected, reports)
}

func TestDetectDriftUnsupportedAttribute(t *testing.T) {
	stateInstances := []parser.Instance{{
		IndexKey: "app-unsupported-attr",
		Attributes: parser.InstanceAttributes{
			ID: "i-unsupported",
		},
	}}

	configs := []*parser.InstanceConfig{{
		ApplicationName: "app-unsupported-attr",
	}}

	attributes := []string{"unsupported_attr"}

	reports := comparator.DetectDrift(stateInstances, configs, attributes)
	assert.Empty(t, reports)
}

func TestDetectDriftSourceDestCheckNilSkipped(t *testing.T) {
	stateInstances := []parser.Instance{{
		IndexKey: "app-srcdst-nilcheck",
		Attributes: parser.InstanceAttributes{
			ID:              "i-srcdst-1",
			SourceDestCheck: false,
		},
	}}

	configs := []*parser.InstanceConfig{{
		ApplicationName: "app-srcdst",
		// SourceDestCheck is intentionally nil to trigger the skip logic
	}}

	attributes := []string{"source_dest_check"}

	reports := comparator.DetectDrift(stateInstances, configs, attributes)

	assert.Empty(t, reports, "Should skip source_dest_check if config value is nil")
}

func TestDetectDriftSecurityGroupsLengthMismatch(t *testing.T) {
	stateInstances := []parser.Instance{{
		IndexKey: "app-sg-len-123",
		Attributes: parser.InstanceAttributes{
			ID:             "i-sg-1",
			SecurityGroups: []string{"sg-1", "sg-2", "sg-3"},
		},
	}}
	configs := []*parser.InstanceConfig{{
		ApplicationName: "app-sg-len",
		SecurityGroups:  []string{"sg-1", "sg-2"},
	}}
	attributes := []string{"security_groups"}

	reports := comparator.DetectDrift(stateInstances, configs, attributes)

	expected := []comparator.DriftReport{{
		InstanceID:      "i-sg-1",
		ApplicationName: "app-sg-len",
		Drifts: []comparator.DriftDetail{
			{
				Attribute:     "security_groups",
				ExpectedValue: []string{"sg-1", "sg-2"},
				ActualValue:   []string{"sg-1", "sg-2", "sg-3"},
			},
		},
	}}
	assert.Equal(t, expected, reports)
}

func TestDetectDriftSecurityGroupsElementMismatch(t *testing.T) {
	stateInstances := []parser.Instance{{
		IndexKey: "app-sg-elem-123",
		Attributes: parser.InstanceAttributes{
			ID:             "i-sg-2",
			SecurityGroups: []string{"sg-A", "sg-B", "sg-C"},
		},
	}}
	configs := []*parser.InstanceConfig{{
		ApplicationName: "app-sg-elem",
		SecurityGroups:  []string{"sg-A", "sg-X", "sg-C"},
	}}
	attributes := []string{"security_groups"}

	reports := comparator.DetectDrift(stateInstances, configs, attributes)

	expected := []comparator.DriftReport{{
		InstanceID:      "i-sg-2",
		ApplicationName: "app-sg-elem",
		Drifts: []comparator.DriftDetail{
			{
				Attribute:     "security_groups",
				ExpectedValue: "sg-X",
				ActualValue:   "sg-B",
			},
		},
	}}
	assert.Equal(t, expected, reports)
}

func TestDetectDriftVPCSecurityGroupIDsLengthMismatch(t *testing.T) {
	stateInstances := []parser.Instance{{
		IndexKey: "app-vpc-sg-len-001",
		Attributes: parser.InstanceAttributes{
			ID:                  "i-vpc-sg-1",
			VPCSecurityGroupIDs: []string{"vpc-sg-1", "vpc-sg-2", "vpc-sg-3"},
		},
	}}
	configs := []*parser.InstanceConfig{{
		ApplicationName:     "app-vpc-sg-len",
		VPCSecurityGroupIDs: []string{"vpc-sg-1", "vpc-sg-2"},
	}}
	attributes := []string{"vpc_security_group_ids"}

	reports := comparator.DetectDrift(stateInstances, configs, attributes)

	expected := []comparator.DriftReport{{
		InstanceID:      "i-vpc-sg-1",
		ApplicationName: "app-vpc-sg-len",
		Drifts: []comparator.DriftDetail{{
			Attribute:     "vpc_security_group_ids",
			ExpectedValue: []string{"vpc-sg-1", "vpc-sg-2"},
			ActualValue:   []string{"vpc-sg-1", "vpc-sg-2", "vpc-sg-3"},
		}},
	}}
	assert.Equal(t, expected, reports)
}

func TestDetectDriftVPCSecurityGroupIDsElementMismatch(t *testing.T) {
	stateInstances := []parser.Instance{{
		IndexKey: "app-vpc-sg-elem-002",
		Attributes: parser.InstanceAttributes{
			ID:                  "i-vpc-sg-2",
			VPCSecurityGroupIDs: []string{"vpc-A", "vpc-B", "vpc-C"},
		},
	}}
	configs := []*parser.InstanceConfig{{
		ApplicationName:     "app-vpc-sg-elem",
		VPCSecurityGroupIDs: []string{"vpc-A", "vpc-X", "vpc-C"},
	}}
	attributes := []string{"vpc_security_group_ids"}

	reports := comparator.DetectDrift(stateInstances, configs, attributes)

	expected := []comparator.DriftReport{{
		InstanceID:      "i-vpc-sg-2",
		ApplicationName: "app-vpc-sg-elem",
		Drifts: []comparator.DriftDetail{{
			Attribute:     "vpc_security_group_ids",
			ExpectedValue: "vpc-X",
			ActualValue:   "vpc-B",
		}},
	}}
	assert.Equal(t, expected, reports)
}

func TestDetectDriftDefaultCaseIsNoOp(t *testing.T) {
	stateInstances := []parser.Instance{{
		IndexKey: "app-default-case-001",
		Attributes: parser.InstanceAttributes{
			ID:                  "i-default-1",
			InstanceType:        "t2.small",
			AMI:                 "ami-abc",
			SecurityGroups:      []string{"sg-1"},
			VPCSecurityGroupIDs: []string{"vpc-1"},
			SubnetID:            "subnet-1",
			SourceDestCheck:     true,
		},
	}}

	configs := []*parser.InstanceConfig{{
		ApplicationName:     "app-default-case",
		InstanceType:        "t2.small",
		AMI:                 "ami-abc",
		SecurityGroups:      []string{"sg-1"},
		VPCSecurityGroupIDs: []string{"vpc-1"},
		SubnetID:            ptrString("subnet-1"),
		SourceDestCheck:     ptrBool(true),
	}}

	attributes := []string{
		"foo_bar_baz",
		"another_one",
		"yetanother",
	}

	reports := comparator.DetectDrift(stateInstances, configs, attributes)
	assert.Empty(t, reports, "Any attribute in the default branch should be skipped without error or drift")
}

func ptrString(s string) *string { return &s }
func ptrBool(b bool) *bool       { return &b }
