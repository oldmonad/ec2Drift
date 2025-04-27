package aws_test

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/oldmonad/ec2Drift/pkg/config/cloud/aws"
	"github.com/oldmonad/ec2Drift/pkg/logger"
	"github.com/stretchr/testify/assert"
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

func TestValidate(t *testing.T) {
	observedCore, recordedLogs := observer.New(zap.ErrorLevel)
	observedLogger := zap.New(observedCore)

	t.Run("missing access key should log error", func(t *testing.T) {
		logger.SetLogger(observedLogger)
		defer logger.SetLogger(zap.NewNop())

		cfg := &awsConfig.Config{
			SecretKey: "secret",
			Region:    "region",
		}

		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "AWS_ACCESS_KEY_ID")

		assert.Equal(t, 1, recordedLogs.Len())
		logEntry := recordedLogs.All()[0]
		assert.Equal(t, "Missing AWS credentials: AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, or AWS_REGION", logEntry.Message)
		assert.Equal(t, zap.ErrorLevel, logEntry.Level)
	})

	t.Run("valid config should not log", func(t *testing.T) {
		core, recorded := observer.New(zap.ErrorLevel)
		logger.SetLogger(zap.New(core))
		defer logger.SetLogger(zap.NewNop())

		cfg := &awsConfig.Config{
			AccessKey: "access",
			SecretKey: "secret",
			Region:    "region",
		}

		err := cfg.Validate()
		assert.NoError(t, err)
		assert.Equal(t, 0, recorded.Len(), "Should not log any errors")
	})

	t.Run("missing secret key", func(t *testing.T) {
		core, recorded := observer.New(zap.ErrorLevel)
		logger.SetLogger(zap.New(core))
		defer logger.SetLogger(zap.NewNop())

		cfg := &awsConfig.Config{AccessKey: "access", Region: "region"}
		err := cfg.Validate()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "AWS_SECRET_ACCESS_KEY")
		assert.Equal(t, 1, recorded.Len())
	})

	t.Run("missing region", func(t *testing.T) {
		core, recorded := observer.New(zap.ErrorLevel)
		logger.SetLogger(zap.New(core))
		defer logger.SetLogger(zap.NewNop())

		cfg := &awsConfig.Config{AccessKey: "access", SecretKey: "secret"}
		err := cfg.Validate()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "AWS_REGION")
		assert.Equal(t, 1, recorded.Len())
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
		if !ok {
			t.Fatalf("Expected aws.Credentials type, got %T", result)
		}

		assert.Equal(t, "AKIAEXAMPLE", creds.AccessKeyID)
		assert.Equal(t, "SecretKeyExample", creds.SecretAccessKey)
		assert.Equal(t, "SessionTokenExample", creds.SessionToken)
	})

	t.Run("credentials without session token", func(t *testing.T) {
		cfg := &awsConfig.Config{
			AccessKey: "AKIAEXAMPLE",
			SecretKey: "SecretKeyExample",
		}

		result := cfg.GetCredentials()
		creds := result.(aws.Credentials)

		assert.Equal(t, "AKIAEXAMPLE", creds.AccessKeyID)
		assert.Equal(t, "SecretKeyExample", creds.SecretAccessKey)
		assert.Empty(t, creds.SessionToken, "Session token should be empty")
	})

	t.Run("partial credentials", func(t *testing.T) {
		cfg := &awsConfig.Config{
			AccessKey:    "AKIAEXAMPLE",
			SessionToken: "SessionTokenExample",
		}

		result := cfg.GetCredentials()
		creds := result.(aws.Credentials)

		assert.Equal(t, "AKIAEXAMPLE", creds.AccessKeyID)
		assert.Empty(t, creds.SecretAccessKey, "Secret key should be empty")
		assert.Equal(t, "SessionTokenExample", creds.SessionToken)
	})

	t.Run("empty credentials", func(t *testing.T) {
		cfg := &awsConfig.Config{}

		result := cfg.GetCredentials()
		creds := result.(aws.Credentials)

		assert.Empty(t, creds.AccessKeyID, "Access key should be empty")
		assert.Empty(t, creds.SecretAccessKey, "Secret key should be empty")
		assert.Empty(t, creds.SessionToken, "Session token should be empty")
	})
}

func TestGetRegion(t *testing.T) {
	cfg := &awsConfig.Config{
		Region: "us-east-1",
	}

	assert.Equal(t, "us-east-1", cfg.GetRegion())
}
