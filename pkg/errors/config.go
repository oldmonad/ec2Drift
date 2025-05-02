package errors

import (
	"fmt"
)

// ErrAWSConfigValidation is returned when AWS provider config fails Validate().
type ErrAWSConfigValidation struct {
	Err error
}

func (e ErrAWSConfigValidation) Error() string {
	return fmt.Sprintf("aws config validation failed: %v", e.Err)
}

func (e ErrAWSConfigValidation) Unwrap() error {
	return e.Err
}

func NewAWSConfigValidation(err error) error {
	return ErrAWSConfigValidation{Err: err}
}

// ErrGCPConfigValidation is returned when GCP provider config fails Validate().
type ErrGCPConfigValidation struct {
	Err error
}

func (e ErrGCPConfigValidation) Error() string {
	return fmt.Sprintf("gcp config validation failed: %v", e.Err)
}

func (e ErrGCPConfigValidation) Unwrap() error {
	return e.Err
}

func NewGCPConfigValidation(err error) error {
	return ErrGCPConfigValidation{Err: err}
}

// ErrUnsupportedProvider is returned when the provider string is unknown.
type ErrUnsupportedProvider struct {
	ProviderType string
}

func (e ErrUnsupportedProvider) Error() string {
	return fmt.Sprintf("unsupported provider: %s", e.ProviderType)
}

func NewUnsupportedProvider(pt string) error {
	return ErrUnsupportedProvider{ProviderType: pt}
}

// ErrDebugParse wraps failures parsing the DEBUG env var.
type ErrDebugParse struct {
	RawValue string
	Err      error
}

func (e ErrDebugParse) Error() string {
	return fmt.Sprintf("failed to parse DEBUG=%q: %v", e.RawValue, e.Err)
}

func (e ErrDebugParse) Unwrap() error {
	return e.Err
}

func NewErrDebugParse(raw string, err error) error {
	return ErrDebugParse{RawValue: raw, Err: err}
}

// ErrMissingCloudProvider is returned when CLOUD_PROVIDER is unset.
type ErrMissingCloudProvider struct{}

func (e ErrMissingCloudProvider) Error() string {
	return fmt.Sprintf("CLOUD_PROVIDER environment variable is required")
}

func NewErrMissingCloudProvider() error {
	return ErrMissingCloudProvider{}
}

// ErrPortParse wraps failures parsing HTTP_PORT.
type ErrPortParse struct {
	RawValue string
	Err      error
}

func (e ErrPortParse) Error() string {
	return fmt.Sprintf("invalid HTTP_PORT=%q: %v", e.RawValue, e.Err)
}

func (e ErrPortParse) Unwrap() error {
	return e.Err
}

func NewErrPortParse(raw string, err error) error {
	return ErrPortParse{RawValue: raw, Err: err}
}

// ErrPortOutOfRange indicates HTTP_PORT is outside 1–65535.
type ErrPortOutOfRange struct {
	Port int
}

func (e ErrPortOutOfRange) Error() string {
	return fmt.Sprintf("HTTP_PORT out of bounds: %d (must be 1–65535)", e.Port)
}

func NewErrPortOutOfRange(port int) error {
	return ErrPortOutOfRange{Port: port}
}

// ErrMissingPaths is returned when STATE_PATH or OUTPUT_PATH are unset.
type ErrMissingPaths struct{}

func (e ErrMissingPaths) Error() string {
	return "STATE_PATH is required"
}

func NewErrMissingPaths() error {
	return ErrMissingPaths{}
}

// ErrCloudConfigNotInit indicates loadCloudConfig wasn’t called or failed.
type ErrCloudConfigNotInit struct{}

func (e ErrCloudConfigNotInit) Error() string {
	return "cloud configuration not initialized"
}

func NewErrCloudConfigNotInit() error {
	return ErrCloudConfigNotInit{}
}

// ErrLoadGeneralConfig wraps any other loadGeneralConfig failures.
type ErrLoadGeneralConfig struct {
	Err error
}

func (e ErrLoadGeneralConfig) Error() string {
	return fmt.Sprintf("failed to load general configurations: %v", e.Err)
}

func (e ErrLoadGeneralConfig) Unwrap() error {
	return e.Err
}

func NewErrLoadGeneralConfig(err error) error {
	return ErrLoadGeneralConfig{Err: err}
}

// ErrLoadCloudConfig wraps loadCloudConfig failures.
type ErrLoadCloudConfig struct {
	Err error
}

func (e ErrLoadCloudConfig) Error() string {
	return fmt.Sprintf("failed to load cloud configuration: %v", e.Err)
}

func (e ErrLoadCloudConfig) Unwrap() error {
	return e.Err
}

func NewErrLoadCloudConfig(err error) error {
	return ErrLoadCloudConfig{Err: err}
}

// ErrInvalidConfigurations wraps validateGeneralConfig failures.
type ErrInvalidConfigurations struct {
	Err error
}

func (e ErrInvalidConfigurations) Error() string {
	return fmt.Sprintf("invalid configurations: %v", e.Err)
}

func (e ErrInvalidConfigurations) Unwrap() error {
	return e.Err
}

func NewErrInvalidConfigurations(err error) error {
	return ErrInvalidConfigurations{Err: err}
}

// ErrMissingCredentials is returned when any of AWS_ACCESS_KEY_ID,
// AWS_SECRET_ACCESS_KEY, or AWS_REGION is not set.
type ErrMissingCredentials struct {
	Missing []string
}

func (e ErrMissingCredentials) Error() string {
	return fmt.Sprintf(
		"missing AWS credentials: %s",
		e.Missing,
	)
}

// NewErrMissingCredentials constructs an ErrMissingCredentials listing which
// environment variables were empty.
func NewErrMissingCredentials(missing []string) error {
	return ErrMissingCredentials{Missing: missing}
}

// ErrMissingGCPConfig indicates that one or more required GCP environment
// variables were not set.
type ErrMissingGCPConfig struct {
	Missing []string
}

func (e ErrMissingGCPConfig) Error() string {
	return fmt.Sprintf(
		"missing GCP configuration variables: %s",
		e.Missing,
	)
}

// NewErrMissingGCPConfig constructs an ErrMissingGCPConfig listing which
// environment variables were empty.
func NewErrMissingGCPConfig(missing []string) error {
	return ErrMissingGCPConfig{Missing: missing}
}

type InvalidConfigCredential struct {
	Err string
}

func (e InvalidConfigCredential) Error() string {
	return fmt.Sprintf("invalid configuration credentials: %v", e.Err)
}

func NewInvalidConfigCredential(err string) error {
	return InvalidConfigCredential{Err: err}
}
