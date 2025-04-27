package driftchecker_test

import (
	"context"
	"testing"

	"github.com/oldmonad/ec2Drift/internal/driftchecker"
	"github.com/oldmonad/ec2Drift/pkg/cloud"
	"github.com/stretchr/testify/assert"
)

func createInstance(name, id, ami, instanceType string, securityGroups []string, tags map[string]string, volumeSize int, volumeType string) cloud.Instance {
	inst := cloud.Instance{
		InstanceID:     id,
		AMI:            ami,
		InstanceType:   instanceType,
		SecurityGroups: securityGroups,
		Tags:           tags,
	}
	inst.RootBlockDevice.VolumeSize = volumeSize
	inst.RootBlockDevice.VolumeType = volumeType
	if tags == nil {
		inst.Tags = make(map[string]string)
	}
	inst.Tags["Name"] = name
	return inst
}

func TestDetectBasicDrift(t *testing.T) {
	oldInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-111", "t2.micro", []string{"sg-1"}, nil, 100, "gp2"),
	}
	currentInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-222", "t2.large", []string{"sg-1"}, nil, 100, "gp2"),
	}
	attributes := []string{"ami", "instance_type"}

	reports := driftchecker.Detect(context.Background(), oldInstances, currentInstances, attributes)

	expected := []driftchecker.DriftReport{
		{
			InstanceID: "i-123",
			Name:       "app1",
			Drifts: []driftchecker.DriftDetail{
				{Attribute: "ami", ExpectedValue: "ami-111", ActualValue: "ami-222"},
				{Attribute: "instance_type", ExpectedValue: "t2.micro", ActualValue: "t2.large"},
			},
		},
	}

	assert.ElementsMatch(t, expected, reports)
}

func TestDetectNoDrift(t *testing.T) {
	oldInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-111", "t2.micro", []string{"sg-1"}, nil, 100, "gp2"),
	}
	currentInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-111", "t2.micro", []string{"sg-1"}, nil, 100, "gp2"),
	}
	attributes := []string{"ami", "instance_type"}

	reports := driftchecker.Detect(context.Background(), oldInstances, currentInstances, attributes)
	assert.Empty(t, reports)
}

func TestDetectInstanceAdded(t *testing.T) {
	oldInstances := []cloud.Instance{}
	currentInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-111", "t2.micro", []string{"sg-1"}, nil, 100, "gp2"),
	}
	attributes := []string{"ami"}

	reports := driftchecker.Detect(context.Background(), oldInstances, currentInstances, attributes)

	expected := []driftchecker.DriftReport{
		{
			InstanceID: "i-123",
			Name:       "app1",
			Drifts: []driftchecker.DriftDetail{
				{Attribute: "instance_added", ExpectedValue: nil, ActualValue: currentInstances[0]},
			},
		},
	}

	assert.ElementsMatch(t, expected, reports)
}

func TestDetectInstanceRemoved(t *testing.T) {
	oldInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-111", "t2.micro", []string{"sg-1"}, nil, 100, "gp2"),
	}
	currentInstances := []cloud.Instance{}
	attributes := []string{"ami"}

	reports := driftchecker.Detect(context.Background(), oldInstances, currentInstances, attributes)

	expected := []driftchecker.DriftReport{
		{
			InstanceID: "i-123",
			Name:       "app1",
			Drifts: []driftchecker.DriftDetail{
				{Attribute: "instance_removed", ExpectedValue: oldInstances[0], ActualValue: nil},
			},
		},
	}

	assert.ElementsMatch(t, expected, reports)
}

func TestDetectSecurityGroupsNoDrift(t *testing.T) {
	oldInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-111", "t2.micro", []string{"sg-1", "sg-2"}, nil, 100, "gp2"),
	}
	currentInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-111", "t2.micro", []string{"sg-2", "sg-1"}, nil, 100, "gp2"),
	}
	attributes := []string{"security_groups"}

	reports := driftchecker.Detect(context.Background(), oldInstances, currentInstances, attributes)
	assert.Empty(t, reports)
}

func TestDetectSecurityGroupsDrift(t *testing.T) {
	oldInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-111", "t2.micro", []string{"sg-1", "sg-2"}, nil, 100, "gp2"),
	}
	currentInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-111", "t2.micro", []string{"sg-3", "sg-4"}, nil, 100, "gp2"),
	}
	attributes := []string{"security_groups"}

	reports := driftchecker.Detect(context.Background(), oldInstances, currentInstances, attributes)

	expected := []driftchecker.DriftReport{
		{
			InstanceID: "i-123",
			Name:       "app1",
			Drifts: []driftchecker.DriftDetail{
				{
					Attribute:     "security_groups",
					ExpectedValue: []string{"sg-1", "sg-2"},
					ActualValue:   []string{"sg-3", "sg-4"},
				},
			},
		},
	}

	assert.ElementsMatch(t, expected, reports)
}

func TestDetectTagsDrift(t *testing.T) {
	oldTags := map[string]string{"Env": "prod", "Owner": "teamA"}
	oldInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-111", "t2.micro", nil, oldTags, 100, "gp2"),
	}
	currentTags := map[string]string{"Env": "dev", "Owner": "teamA"}
	currentInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-111", "t2.micro", nil, currentTags, 100, "gp2"),
	}
	attributes := []string{"tags.Env"}

	reports := driftchecker.Detect(context.Background(), oldInstances, currentInstances, attributes)

	expected := []driftchecker.DriftReport{
		{
			InstanceID: "i-123",
			Name:       "app1",
			Drifts: []driftchecker.DriftDetail{
				{
					Attribute:     "tags.Env",
					ExpectedValue: "prod",
					ActualValue:   "dev",
				},
			},
		},
	}

	assert.ElementsMatch(t, expected, reports)
}

func TestDetectRootBlockDeviceVolumeSizeDrift(t *testing.T) {
	oldInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-111", "t2.micro", nil, nil, 100, "gp2"),
	}
	currentInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-111", "t2.micro", nil, nil, 200, "gp2"),
	}
	attributes := []string{"root_block_device.volume_size"}

	reports := driftchecker.Detect(context.Background(), oldInstances, currentInstances, attributes)

	expected := []driftchecker.DriftReport{
		{
			InstanceID: "i-123",
			Name:       "app1",
			Drifts: []driftchecker.DriftDetail{
				{
					Attribute:     "root_block_device.volume_size",
					ExpectedValue: 100,
					ActualValue:   200,
				},
			},
		},
	}

	assert.ElementsMatch(t, expected, reports)
}

func TestDetectConcurrentProcessing(t *testing.T) {
	oldInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-111", "t2.micro", nil, nil, 100, "gp2"),
		createInstance("app2", "i-456", "ami-222", "t2.small", nil, nil, 200, "io1"),
	}
	currentInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-333", "t2.large", nil, nil, 100, "gp2"),
		createInstance("app2", "i-456", "ami-222", "t2.small", nil, nil, 200, "io1"),
	}
	attributes := []string{"ami", "instance_type"}

	reports := driftchecker.Detect(context.Background(), oldInstances, currentInstances, attributes)

	expected := []driftchecker.DriftReport{
		{
			InstanceID: "i-123",
			Name:       "app1",
			Drifts: []driftchecker.DriftDetail{
				{Attribute: "ami", ExpectedValue: "ami-111", ActualValue: "ami-333"},
				{Attribute: "instance_type", ExpectedValue: "t2.micro", ActualValue: "t2.large"},
			},
		},
	}

	assert.ElementsMatch(t, expected, reports)
}

func TestDetectContextCancellation(t *testing.T) {
	oldInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-111", "t2.micro", nil, nil, 100, "gp2"),
	}
	currentInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-222", "t2.large", nil, nil, 100, "gp2"),
	}
	attributes := []string{"ami"}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	reports := driftchecker.Detect(ctx, oldInstances, currentInstances, attributes)
	assert.Empty(t, reports)
}

func TestDetectEmptyAttributes(t *testing.T) {
	oldInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-111", "t2.micro", nil, nil, 100, "gp2"),
	}
	currentInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-222", "t2.large", nil, nil, 100, "gp2"),
	}
	attributes := []string{}

	reports := driftchecker.Detect(context.Background(), oldInstances, currentInstances, attributes)
	assert.Empty(t, reports)
}

func TestDetectUnsupportedAttribute(t *testing.T) {
	oldInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-111", "t2.micro", nil, nil, 100, "gp2"),
	}
	currentInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-111", "t2.micro", nil, nil, 100, "gp2"),
	}
	attributes := []string{"unsupported_attr"}

	reports := driftchecker.Detect(context.Background(), oldInstances, currentInstances, attributes)
	assert.Empty(t, reports)
}

func TestDetectNameTagChange(t *testing.T) {
	oldInstances := []cloud.Instance{
		createInstance("app1-old", "i-123", "ami-111", "t2.micro", nil, nil, 100, "gp2"),
	}
	currentInstances := []cloud.Instance{
		createInstance("app1-new", "i-123", "ami-111", "t2.micro", nil, nil, 100, "gp2"),
	}
	attributes := []string{"ami"}

	reports := driftchecker.Detect(context.Background(), oldInstances, currentInstances, attributes)

	expected := []driftchecker.DriftReport{
		{
			InstanceID: "i-123",
			Name:       "app1-old",
			Drifts: []driftchecker.DriftDetail{
				{Attribute: "instance_removed", ExpectedValue: oldInstances[0], ActualValue: nil},
			},
		},
		{
			InstanceID: "i-123",
			Name:       "app1-new",
			Drifts: []driftchecker.DriftDetail{
				{Attribute: "instance_added", ExpectedValue: nil, ActualValue: currentInstances[0]},
			},
		},
	}

	assert.ElementsMatch(t, expected, reports)
}

func TestDetectEmptyOldState(t *testing.T) {
	var oldInstances []cloud.Instance
	currentInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-111", "t2.micro", nil, nil, 100, "gp2"),
	}
	attributes := []string{"ami"}

	reports := driftchecker.Detect(context.Background(), oldInstances, currentInstances, attributes)

	expected := []driftchecker.DriftReport{
		{
			InstanceID: "i-123",
			Name:       "app1",
			Drifts: []driftchecker.DriftDetail{
				{Attribute: "instance_added", ExpectedValue: nil, ActualValue: currentInstances[0]},
			},
		},
	}

	assert.ElementsMatch(t, expected, reports)
}

func TestDetectEmptyCurrentState(t *testing.T) {
	oldInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-111", "t2.micro", nil, nil, 100, "gp2"),
	}
	var currentInstances []cloud.Instance
	attributes := []string{"ami"}

	reports := driftchecker.Detect(context.Background(), oldInstances, currentInstances, attributes)

	expected := []driftchecker.DriftReport{
		{
			InstanceID: "i-123",
			Name:       "app1",
			Drifts: []driftchecker.DriftDetail{
				{Attribute: "instance_removed", ExpectedValue: oldInstances[0], ActualValue: nil},
			},
		},
	}

	assert.ElementsMatch(t, expected, reports)
}

func TestDetectTagsDriftAllTags(t *testing.T) {
	oldTags := map[string]string{"Env": "prod", "Owner": "teamA"}
	oldInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-111", "t2.micro", nil, oldTags, 100, "gp2"),
	}
	currentTags := map[string]string{"Env": "prod"}
	currentInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-111", "t2.micro", nil, currentTags, 100, "gp2"),
	}
	attributes := []string{"tags"}

	reports := driftchecker.Detect(context.Background(), oldInstances, currentInstances, attributes)

	expected := []driftchecker.DriftReport{
		{
			InstanceID: "i-123",
			Name:       "app1",
			Drifts: []driftchecker.DriftDetail{
				{
					Attribute:     "tags.Owner",
					ExpectedValue: "teamA",
					ActualValue:   "",
				},
			},
		},
	}

	assert.ElementsMatch(t, expected, reports)
}

func TestDetectRootBlockDeviceDriftBothAttributes(t *testing.T) {
	oldInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-111", "t2.micro", nil, nil, 100, "gp2"),
	}
	currentInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-111", "t2.micro", nil, nil, 200, "gp3"),
	}
	attributes := []string{"root_block_device"}

	reports := driftchecker.Detect(context.Background(), oldInstances, currentInstances, attributes)

	expectedDrifts := []driftchecker.DriftDetail{
		{Attribute: "root_block_device.volume_size", ExpectedValue: 100, ActualValue: 200},
		{Attribute: "root_block_device.volume_type", ExpectedValue: "gp2", ActualValue: "gp3"},
	}

	assert.Len(t, reports, 1, "Expected one drift report")
	assert.ElementsMatch(t, expectedDrifts, reports[0].Drifts, "Drifts for volume size and type should be detected")
}

func TestDetectRootBlockDeviceVolumeTypeDrift(t *testing.T) {
	oldInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-111", "t2.micro", nil, nil, 100, "gp2"),
	}
	currentInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-111", "t2.micro", nil, nil, 100, "gp3"),
	}
	attributes := []string{"root_block_device.volume_type"}

	reports := driftchecker.Detect(context.Background(), oldInstances, currentInstances, attributes)

	expected := []driftchecker.DriftReport{
		{
			InstanceID: "i-123",
			Name:       "app1",
			Drifts: []driftchecker.DriftDetail{
				{
					Attribute:     "root_block_device.volume_type",
					ExpectedValue: "gp2",
					ActualValue:   "gp3",
				},
			},
		},
	}

	assert.ElementsMatch(t, expected, reports)
}

func TestDetectSecurityGroupsDriftDifferentLength(t *testing.T) {
	oldInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-111", "t2.micro", []string{"sg-1", "sg-2"}, nil, 100, "gp2"),
	}
	currentInstances := []cloud.Instance{
		createInstance("app1", "i-123", "ami-111", "t2.micro", []string{"sg-1"}, nil, 100, "gp2"),
	}
	attributes := []string{"security_groups"}

	reports := driftchecker.Detect(context.Background(), oldInstances, currentInstances, attributes)

	expectedDrift := driftchecker.DriftDetail{
		Attribute:     "security_groups",
		ExpectedValue: []string{"sg-1", "sg-2"},
		ActualValue:   []string{"sg-1"},
	}

	assert.Len(t, reports, 1, "Expected one drift report")
	assert.Contains(t, reports[0].Drifts, expectedDrift, "Security groups with different lengths should be reported as drifted")
}
