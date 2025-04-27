package cloud

import (
	"context"

	"github.com/oldmonad/ec2Drift/pkg/config/cloud"
)

type Instance struct {
	InstanceID      string            `json:"instance_id"`
	AMI             string            `json:"ami"`
	InstanceType    string            `json:"instance_type"`
	SecurityGroups  []string          `json:"security_groups"`
	Tags            map[string]string `json:"tags"`
	RootBlockDevice struct {
		VolumeSize int    `json:"volume_size"`
		VolumeType string `json:"volume_type"`
	} `json:"root_block_device"`
}

type CloudProvider interface {
	FetchInstances(ctx context.Context, cfg cloud.ProviderConfig) ([]Instance, error)
}
