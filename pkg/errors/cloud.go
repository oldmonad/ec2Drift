package errors

import (
	"fmt"
)

// ErrWrongConfigType indicates the passed-in ProviderConfig wasn't *aws.Config.
type ErrWrongConfigType struct {
	Got interface{}
}

func (e ErrWrongConfigType) Error() string {
	return fmt.Sprintf("unexpected provider config type %T, want *aws.Config", e.Got)
}

func NewWrongConfigType(got interface{}) error {
	return ErrWrongConfigType{Got: got}
}

// ErrAWSConfigLoad wraps failures loading AWS SDK config.
type ErrAWSConfigLoad struct {
	Err error
}

func (e ErrAWSConfigLoad) Error() string {
	return fmt.Sprintf("unable to load AWS SDK config: %v", e.Err)
}

func (e ErrAWSConfigLoad) Unwrap() error {
	return e.Err
}

func NewAWSConfigLoad(err error) error {
	return ErrAWSConfigLoad{Err: err}
}

// ErrDescribeInstances wraps failures in DescribeInstances.
type ErrDescribeInstances struct {
	Err error
}

func (e ErrDescribeInstances) Error() string {
	return fmt.Sprintf("failed to describe instances, make sure your AWS credentials have not timed out: %v", e.Err)
}

func (e ErrDescribeInstances) Unwrap() error {
	return e.Err
}

func NewDescribeInstances(err error) error {
	return ErrDescribeInstances{Err: err}
}

// ErrDescribeVolumes wraps failures or empty results in DescribeVolumes.
type ErrDescribeVolumes struct {
	VolumeID string
	Err      error
}

func (e ErrDescribeVolumes) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("failed to describe volume %s: %v", e.VolumeID, e.Err)
	}
	return fmt.Sprintf("no volumes found for %s", e.VolumeID)
}

func (e ErrDescribeVolumes) Unwrap() error {
	return e.Err
}

func NewDescribeVolumes(volID string, err error) error {
	return ErrDescribeVolumes{VolumeID: volID, Err: err}
}

// ErrMapInstance covers any unexpected mapping failure.
type ErrMapInstance struct {
	InstanceID string
	Reason     string
}

func (e ErrMapInstance) Error() string {
	return fmt.Sprintf("failed to map EC2 instance %s: %s", e.InstanceID, e.Reason)
}

func NewMapInstance(id, reason string) error {
	return ErrMapInstance{InstanceID: id, Reason: reason}
}
