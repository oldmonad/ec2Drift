package aws_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/oldmonad/ec2Drift/pkg/cloud"
	awsProvider "github.com/oldmonad/ec2Drift/pkg/cloud/aws"
	config "github.com/oldmonad/ec2Drift/pkg/config/cloud"
	awsConfig "github.com/oldmonad/ec2Drift/pkg/config/cloud/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type ProviderConfigMock struct{}

func (m *ProviderConfigMock) GetRegion() string {
	return ""
}

func (m *ProviderConfigMock) GetCredentials() interface{} {
	return nil
}

func (m *ProviderConfigMock) Validate() error {
	return nil
}

type MockEC2Client struct {
	mock.Mock
}

func (m *MockEC2Client) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	args := m.Called(ctx, params)

	var out *ec2.DescribeInstancesOutput
	if tmp := args.Get(0); tmp != nil {
		out = tmp.(*ec2.DescribeInstancesOutput)
	}
	return out, args.Error(1)
}

func (m *MockEC2Client) DescribeVolumes(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	args := m.Called(ctx, params)
	var out *ec2.DescribeVolumesOutput
	if tmp := args.Get(0); tmp != nil {
		out = tmp.(*ec2.DescribeVolumesOutput)
	}
	return out, args.Error(1)
}

func TestAWSProviderFetchInstances(t *testing.T) {
	validConfig := &awsConfig.Config{
		AccessKey:    "test-key",
		SecretKey:    "test-secret",
		SessionToken: "test-token",
		Region:       "us-west-2",
	}

	testCases := []struct {
		name        string
		config      config.ProviderConfig
		mockSetup   func(*MockEC2Client)
		expected    []cloud.Instance
		expectedErr string
	}{
		{
			name:   "successful instance retrieval",
			config: validConfig,
			mockSetup: func(m *MockEC2Client) {
				instance1 := createTestInstance("i-123", "ami-123", "t2.micro", []string{"sg-1"}, map[string]string{"Name": "test"}, "vol-123", "/dev/sda1")
				instance2 := createTestInstance("i-456", "ami-456", "m5.large", []string{"sg-2"}, map[string]string{"Env": "prod"}, "", "")
				volume := &types.Volume{Size: aws.Int32(100), VolumeType: types.VolumeTypeGp2}

				m.On("DescribeInstances", context.Background(), &ec2.DescribeInstancesInput{}).
					Return(&ec2.DescribeInstancesOutput{
						Reservations: []types.Reservation{{Instances: []types.Instance{instance1}}},
						NextToken:    aws.String("token"),
					}, nil).Once()

				m.On("DescribeInstances", context.Background(), &ec2.DescribeInstancesInput{NextToken: aws.String("token")}).
					Return(&ec2.DescribeInstancesOutput{
						Reservations: []types.Reservation{{Instances: []types.Instance{instance2}}},
					}, nil).Once()

				m.On("DescribeVolumes", context.Background(), &ec2.DescribeVolumesInput{VolumeIds: []string{"vol-123"}}).
					Return(&ec2.DescribeVolumesOutput{Volumes: []types.Volume{*volume}}, nil).Once()
			},
			expected: []cloud.Instance{
				{
					InstanceID:     "i-123",
					AMI:            "ami-123",
					InstanceType:   "t2.micro",
					SecurityGroups: []string{"sg-1"},
					Tags:           map[string]string{"Name": "test"},
					RootBlockDevice: struct {
						VolumeSize int    `json:"volume_size"`
						VolumeType string `json:"volume_type"`
					}{VolumeSize: 100, VolumeType: "gp2"},
				},
				{
					InstanceID:     "i-456",
					AMI:            "ami-456",
					InstanceType:   "m5.large",
					SecurityGroups: []string{"sg-2"},
					Tags:           map[string]string{"Env": "prod"},
					RootBlockDevice: struct {
						VolumeSize int    `json:"volume_size"`
						VolumeType string `json:"volume_type"`
					}{},
				},
			},
		},
		{
			name:        "invalid provider config",
			config:      &ProviderConfigMock{},
			mockSetup:   func(m *MockEC2Client) {},
			expectedErr: "unexpected provider config type",
		},
		{
			name:   "aws api error",
			config: validConfig,
			mockSetup: func(m *MockEC2Client) {
				m.On("DescribeInstances", context.Background(), &ec2.DescribeInstancesInput{}).
					Return(nil, errors.New("api error")).Once()
			},
			expectedErr: "failed to describe instances",
		},
		{
			name:   "volume retrieval error",
			config: validConfig,
			mockSetup: func(m *MockEC2Client) {
				instance := createTestInstance("i-789", "ami-789", "t2.small", nil, nil, "vol-err", "/dev/sda1")
				m.On("DescribeInstances", context.Background(), &ec2.DescribeInstancesInput{}).
					Return(&ec2.DescribeInstancesOutput{
						Reservations: []types.Reservation{{Instances: []types.Instance{instance}}},
					}, nil).Once()
				m.On("DescribeVolumes", context.Background(), &ec2.DescribeVolumesInput{VolumeIds: []string{"vol-err"}}).
					Return(nil, errors.New("volume error")).Once()
			},
			expected: []cloud.Instance{
				{
					InstanceID:     "i-789",
					AMI:            "ami-789",
					InstanceType:   "t2.small",
					SecurityGroups: []string{},
					Tags:           map[string]string{},
					RootBlockDevice: struct {
						VolumeSize int    `json:"volume_size"`
						VolumeType string `json:"volume_type"`
					}{},
				},
			},
		},
		{
			name:   "empty instance data",
			config: validConfig,
			mockSetup: func(m *MockEC2Client) {
				m.On("DescribeInstances", context.Background(), &ec2.DescribeInstancesInput{}).
					Return(&ec2.DescribeInstancesOutput{}, nil).Once()
			},
			expected: []cloud.Instance{},
		},
		{
			name: "config loading error",
			config: &awsConfig.Config{
				AccessKey:    "invalid-key",
				SecretKey:    "invalid-secret",
				SessionToken: "invalid-token",
				Region:       "invalid-region",
			},
			mockSetup:   func(m *MockEC2Client) {},
			expectedErr: "failed to describe instances",
		},
		{
			name:   "client initialization success",
			config: validConfig,
			mockSetup: func(m *MockEC2Client) {
				m.On("DescribeInstances", context.Background(), &ec2.DescribeInstancesInput{}).
					Return(&ec2.DescribeInstancesOutput{}, nil).Once()
			},
			expected: []cloud.Instance{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockEC2 := new(MockEC2Client)
			provider := awsProvider.NewAWSProvider()
			if tc.name != "config loading error" {
				provider.SetEC2Client(mockEC2)
			}

			tc.mockSetup(mockEC2)

			instances, err := provider.FetchInstances(context.Background(), tc.config)

			if tc.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr)
				return
			}
			require.NoError(t, err)

			assert.Equal(t, tc.expected, instances)
			if tc.name != "config loading error" {
				mockEC2.AssertExpectations(t)
			}
		})
	}
}

func createTestInstance(
	id, ami, instanceType string,
	securityGroups []string,
	tags map[string]string,
	volumeID, deviceName string,
) types.Instance {
	instance := types.Instance{
		InstanceId:   aws.String(id),
		ImageId:      aws.String(ami),
		InstanceType: types.InstanceType(instanceType),
	}

	for _, sg := range securityGroups {
		instance.SecurityGroups = append(instance.SecurityGroups, types.GroupIdentifier{
			GroupName: aws.String(sg),
		})
	}

	for k, v := range tags {
		instance.Tags = append(instance.Tags, types.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		})
	}

	if volumeID != "" {
		instance.BlockDeviceMappings = []types.InstanceBlockDeviceMapping{
			{
				DeviceName: aws.String(deviceName),
				Ebs: &types.EbsInstanceBlockDevice{
					VolumeId: aws.String(volumeID),
				},
			},
		}
		instance.RootDeviceName = aws.String(deviceName)
	}

	return instance
}
