package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsPkgConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/oldmonad/ec2Drift/pkg/cloud"
	config "github.com/oldmonad/ec2Drift/pkg/config/cloud"
	awsConfig "github.com/oldmonad/ec2Drift/pkg/config/cloud/aws"
)

type EC2Client interface {
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	DescribeVolumes(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error)
}

type AWSProvider struct {
	EC2Client EC2Client
}

func NewAWSProvider() *AWSProvider {
	return &AWSProvider{}
}

type EC2Instance struct {
	InstanceID      string
	AMI             string
	InstanceType    string
	SecurityGroups  []string
	Tags            map[string]string
	RootBlockDevice *BlockDevice
}

type BlockDevice struct {
	VolumeID   string
	DeviceName string
	SizeGB     int64
	VolumeType string
}

func (p *AWSProvider) FetchInstances(ctx context.Context, providerCfg config.ProviderConfig) ([]cloud.Instance, error) {

	awsCfgStruct, ok := providerCfg.(*awsConfig.Config)

	if !ok {
		return nil, fmt.Errorf("unexpected provider config type %T, want *aws.Config", providerCfg)
	}

	if p.EC2Client == nil {
		awsCfg, err := awsPkgConfig.LoadDefaultConfig(ctx,
			awsPkgConfig.WithRegion(awsCfgStruct.GetRegion()),
			awsPkgConfig.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(
					awsCfgStruct.AccessKey,
					awsCfgStruct.SecretKey,
					awsCfgStruct.SessionToken,
				),
			),
		)
		if err != nil {
			return nil, fmt.Errorf("unable to load AWS SDK config: %w", err)
		}
		p.EC2Client = ec2.NewFromConfig(awsCfg)
	}

	paginator := ec2.NewDescribeInstancesPaginator(p.EC2Client, &ec2.DescribeInstancesInput{})
	instances := make([]cloud.Instance, 0)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe instances: %w", err)
		}

		for _, reservation := range page.Reservations {
			for _, instance := range reservation.Instances {
				e := mapToEC2Instance(ctx, instance, p.EC2Client)

				var rbd struct {
					VolumeSize int    `json:"volume_size"`
					VolumeType string `json:"volume_type"`
				}
				if e.RootBlockDevice != nil {
					rbd = struct {
						VolumeSize int    `json:"volume_size"`
						VolumeType string `json:"volume_type"`
					}{
						VolumeSize: int(e.RootBlockDevice.SizeGB),
						VolumeType: e.RootBlockDevice.VolumeType,
					}
				}

				instances = append(instances, cloud.Instance{
					InstanceID:      e.InstanceID,
					AMI:             e.AMI,
					InstanceType:    e.InstanceType,
					SecurityGroups:  e.SecurityGroups,
					Tags:            e.Tags,
					RootBlockDevice: rbd,
				})
			}
		}
	}

	return instances, nil
}

func getVolumeDetails(ctx context.Context, client EC2Client, volumeID string) BlockDevice {
	volInput := &ec2.DescribeVolumesInput{
		VolumeIds: []string{volumeID},
	}
	volResult, err := client.DescribeVolumes(ctx, volInput)
	if err != nil || len(volResult.Volumes) == 0 {
		return BlockDevice{VolumeID: volumeID}
	}

	var sizeGB int64
	if volResult.Volumes[0].Size != nil {
		sizeGB = int64(*volResult.Volumes[0].Size)
	}

	return BlockDevice{
		VolumeID:   volumeID,
		SizeGB:     sizeGB,
		VolumeType: string(volResult.Volumes[0].VolumeType),
	}
}

func mapToEC2Instance(ctx context.Context, instance types.Instance, client EC2Client) *EC2Instance {
	e := &EC2Instance{
		InstanceID:     aws.ToString(instance.InstanceId),
		AMI:            aws.ToString(instance.ImageId),
		InstanceType:   string(instance.InstanceType),
		SecurityGroups: make([]string, 0),
		Tags:           make(map[string]string),
	}

	for _, tag := range instance.Tags {
		if e.Tags == nil {
			e.Tags = make(map[string]string)
		}
		e.Tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	for _, sg := range instance.SecurityGroups {
		if e.SecurityGroups == nil {
			e.SecurityGroups = []string{}
		}
		e.SecurityGroups = append(e.SecurityGroups, aws.ToString(sg.GroupName))
	}

	for _, bd := range instance.BlockDeviceMappings {
		if bd.Ebs != nil && aws.ToString(bd.DeviceName) == aws.ToString(instance.RootDeviceName) {
			v := getVolumeDetails(ctx, client, aws.ToString(bd.Ebs.VolumeId))
			e.RootBlockDevice = &BlockDevice{
				VolumeID:   aws.ToString(bd.Ebs.VolumeId),
				DeviceName: aws.ToString(bd.DeviceName),
				SizeGB:     v.SizeGB,
				VolumeType: v.VolumeType,
			}
			break
		}
	}

	return e
}

func (p *AWSProvider) SetEC2Client(c EC2Client) {
	p.EC2Client = c
}
