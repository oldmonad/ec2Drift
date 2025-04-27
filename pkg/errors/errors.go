package errors

import (
	"errors"
	"fmt"
	"path/filepath"
)

var (
	ErrUnexpectedProviderConfigType = errors.New("unexpected provider config type")
	ErrUnableToLoadAWSConfig        = errors.New("unable to load AWS SDK config")
	ErrFailedToDescribeInstances    = errors.New("failed to describe instances")
	ErrDescribeVolumesFailed        = errors.New("failed to describe volumes or no volumes found")
)

type ErrNoEC2Instances struct {
	Path string
}

func (e ErrNoEC2Instances) Error() string {
	return fmt.Sprintf("no AWS EC2 instances found in state file: %s", filepath.Base(e.Path))
}

func NewNoEC2Instances(path string) error {
	return ErrNoEC2Instances{Path: path}
}

type ErrInvalidCloudProvider struct {
	Provider string
}

func (e ErrInvalidCloudProvider) Error() string {
	return fmt.Sprintf("unsupported cloud provider: %s", e.Provider)
}

func NewInvalidCloudProvider(provider string) error {
	return ErrInvalidCloudProvider{Provider: provider}
}

type ErrCloudConfigValidation struct {
	Reason string
}

func (e ErrCloudConfigValidation) Error() string {
	return fmt.Sprintf("cloud configuration validation failed: %s", e.Reason)
}

func NewCloudConfigValidation(reason string) error {
	return ErrCloudConfigValidation{Reason: reason}
}

type ErrReadFile struct {
	Err error
}

func (e ErrReadFile) Error() string {
	return fmt.Sprintf("read file: %v", e.Err)
}

func (e ErrReadFile) Unwrap() error {
	return e.Err
}

func NewReadFileError(err error) error {
	return ErrReadFile{Err: err}
}

type ErrDriftDetected struct{}

func (e ErrDriftDetected) Error() string {
	return "drift detected"
}

func NewDriftDetected() error {
	return ErrDriftDetected{}
}

type ErrParse struct {
	Err error
}

func (e ErrParse) Error() string {
	return fmt.Sprintf("parse error: %v", e.Err)
}

func (e ErrParse) Unwrap() error {
	return e.Err
}
