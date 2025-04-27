package gcp

import (
	"context"

	"github.com/oldmonad/ec2Drift/pkg/cloud"
	config "github.com/oldmonad/ec2Drift/pkg/config/cloud"
)

type GCPProvider struct{}

func (p *GCPProvider) FetchInstances(ctx context.Context, providerCfg config.ProviderConfig) ([]cloud.Instance, error) {
	return []cloud.Instance{
		{
			InstanceID:     "instance-1234567890",
			AMI:            "gcp-image-1234567890",
			InstanceType:   "n1-standard-1",
			SecurityGroups: []string{"gcp-firewall-web", "gcp-ssh"},
			Tags: map[string]string{
				"Name": "GCP-WebServer",
			},
			RootBlockDevice: struct {
				VolumeSize int    `json:"volume_size"`
				VolumeType string `json:"volume_type"`
			}{
				VolumeSize: 10,
				VolumeType: "pd-standard",
			},
		},
	}, nil
}
