package aws_test

import (
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/oldmonad/ec2Drift/pkg/config/cloud/aws"
	"github.com/oldmonad/ec2Drift/pkg/errors"
	"github.com/oldmonad/ec2Drift/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestMain(m *testing.M) {
	logger.SetLogger(zap.NewNop())
	m.Run()
}

func TestLoadConfig(t *testing.T) {
	t.Run("all fields set", func(t *testing.T) {
		t.Setenv("AWS_ACCESS_KEY_ID", "test-access")
		t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")
		t.Setenv("AWS_REGION", "test-region")
		t.Setenv("AWS_SESSION_TOKEN", "test-token")

		cfg := awsConfig.LoadConfig()

		assert.Equal(t, "test-access", cfg.AccessKey)
		assert.Equal(t, "test-secret", cfg.SecretKey)
		assert.Equal(t, "test-region", cfg.Region)
		assert.Equal(t, "test-token", cfg.SessionToken)
	})

	t.Run("optional fields missing", func(t *testing.T) {
		t.Setenv("AWS_ACCESS_KEY_ID", "test-access")
		t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")
		t.Setenv("AWS_REGION", "test-region")

		cfg := awsConfig.LoadConfig()

		assert.Equal(t, "test-access", cfg.AccessKey)
		assert.Equal(t, "test-secret", cfg.SecretKey)
		assert.Equal(t, "test-region", cfg.Region)
		assert.Empty(t, cfg.SessionToken)
	})
}

func TestGetCredentials(t *testing.T) {
	t.Run("full credentials with session token", func(t *testing.T) {
		cfg := &awsConfig.Config{
			AccessKey:    "AKIAEXAMPLE",
			SecretKey:    "SecretKeyExample",
			SessionToken: "SessionTokenExample",
		}

		result := cfg.GetCredentials()
		creds, ok := result.(aws.Credentials)
		assert.True(t, ok, "Should return aws.Credentials type")

		assert.Equal(t, "AKIAEXAMPLE", creds.AccessKeyID)
		assert.Equal(t, "SecretKeyExample", creds.SecretAccessKey)
		assert.Equal(t, "SessionTokenExample", creds.SessionToken)
	})

	t.Run("minimum required credentials", func(t *testing.T) {
		cfg := &awsConfig.Config{
			AccessKey: "AKIAEXAMPLE",
			SecretKey: "SecretKeyExample",
			Region:    "us-west-2",
		}

		result := cfg.GetCredentials()
		creds := result.(aws.Credentials)

		assert.Equal(t, "AKIAEXAMPLE", creds.AccessKeyID)
		assert.Equal(t, "SecretKeyExample", creds.SecretAccessKey)
		assert.Empty(t, creds.SessionToken)
	})
}

func TestGetRegion(t *testing.T) {
	t.Run("returns configured region", func(t *testing.T) {
		cfg := &awsConfig.Config{
			Region: "eu-central-1",
		}

		assert.Equal(t, "eu-central-1", cfg.GetRegion())
	})

	t.Run("empty region returns empty string", func(t *testing.T) {
		cfg := &awsConfig.Config{}
		assert.Empty(t, cfg.GetRegion())
	})
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *awsConfig.Config
		wantErr bool
		missing []string
	}{
		{
			name: "all required fields present",
			config: &awsConfig.Config{
				AccessKey:    "access",
				SecretKey:    "secret",
				Region:       "region",
				SessionToken: "sessiontoken",
			},
			wantErr: false,
			missing: nil,
		},
		{
			name: "missing Session token",
			config: &awsConfig.Config{
				AccessKey: "access",
				SecretKey: "secret",
				Region:    "region",
			},
			wantErr: true,
			missing: []string{"AWS_SESSION_TOKEN"},
		},
		{
			name: "missing Access key id",
			config: &awsConfig.Config{
				SecretKey:    "secret",
				Region:       "region",
				SessionToken: "sessiontoken",
			},
			wantErr: true,
			missing: []string{"AWS_ACCESS_KEY_ID"},
		},
		{
			name: "missing SecretKey",
			config: &awsConfig.Config{
				AccessKey:    "access",
				Region:       "region",
				SessionToken: "sessiontoken",
			},
			wantErr: true,
			missing: []string{"AWS_SECRET_ACCESS_KEY"},
		},
		{
			name: "missing Region",
			config: &awsConfig.Config{
				AccessKey:    "access",
				SecretKey:    "secret",
				SessionToken: "sessiontoken",
			},
			wantErr: true,
			missing: []string{"AWS_REGION"},
		},
		{
			name: "missing AccessKey and SecretKey",
			config: &awsConfig.Config{
				Region:       "region",
				SessionToken: "sessiontoken",
			},
			wantErr: true,
			missing: []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY"},
		},
		{
			name:    "all required fields missing",
			config:  &awsConfig.Config{},
			wantErr: true,
			missing: []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_REGION", "AWS_SESSION_TOKEN"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, recordedLogs := observer.New(zap.ErrorLevel)
			observedLogger := zap.New(core)

			originalLogger := logger.Log
			logger.Log = observedLogger
			defer func() { logger.Log = originalLogger }()

			err := tt.config.Validate()

			if tt.wantErr {
				require.Error(t, err)
				var credsErr errors.ErrMissingCredentials
				require.ErrorAs(t, err, &credsErr)
				assert.ElementsMatch(t, tt.missing, credsErr.Missing)

				require.Equal(t, 1, recordedLogs.Len())
				logEntry := recordedLogs.All()[0]
				assert.Equal(t, "AWS config validation failed", logEntry.Message)
				assert.Equal(t, zap.ErrorLevel, logEntry.Level)

				// Properly handle zap's string array type
				var loggedMissing []string
				for _, field := range logEntry.Context {
					if field.Key == "missing" {
						// Use reflection to safely extract string slice
						val := reflect.ValueOf(field.Interface)
						if val.Kind() == reflect.Slice {
							loggedMissing = make([]string, val.Len())
							for i := 0; i < val.Len(); i++ {
								loggedMissing[i] = val.Index(i).Interface().(string)
							}
						}
						break
					}
				}
				assert.ElementsMatch(t, tt.missing, loggedMissing)
			} else {
				assert.NoError(t, err)
				assert.Zero(t, recordedLogs.Len())
			}
		})
	}
}
