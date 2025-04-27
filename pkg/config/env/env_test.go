package env_test

import (
	"testing"

	"github.com/oldmonad/ec2Drift/pkg/config/env"
	"github.com/oldmonad/ec2Drift/pkg/logger"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestMain(m *testing.M) {
	logger.SetLogger(zap.NewNop())
	m.Run()
}

func TestLoadGeneralConfig(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		expected env.GeneralConfig
	}{
		{
			name: "all fields populated",
			env: map[string]string{
				"DEBUG":       "true",
				"LOG_LEVEL":   "debug",
				"CONFIG_PATH": "/app/config",
				"STATE_PATH":  "/app/state",
				"OUTPUT_PATH": "/app/output",
			},
			expected: env.GeneralConfig{
				DebugMode:  true,
				LogLevel:   "debug",
				ConfigPath: "/app/config",
				StatePath:  "/app/state",
				OutputPath: "/app/output",
			},
		},
		{
			name: "default debug mode",
			env:  map[string]string{},
			expected: env.GeneralConfig{
				DebugMode: false,
			},
		},
		{
			name: "invalid debug mode",
			env: map[string]string{
				"DEBUG": "maybe",
			},
			expected: env.GeneralConfig{
				DebugMode: false,
			},
		},
		{
			name: "partial paths",
			env: map[string]string{
				"STATE_PATH":  "/state",
				"OUTPUT_PATH": "/output",
			},
			expected: env.GeneralConfig{
				StatePath:  "/state",
				OutputPath: "/output",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			defer func() {
				for k := range tt.env {
					t.Setenv(k, "")
				}
			}()

			cfg := env.LoadGeneralConfig()

			assert.Equal(t, tt.expected.DebugMode, cfg.DebugMode)
			assert.Equal(t, tt.expected.LogLevel, cfg.LogLevel)
			assert.Equal(t, tt.expected.ConfigPath, cfg.ConfigPath)
			assert.Equal(t, tt.expected.StatePath, cfg.StatePath)
			assert.Equal(t, tt.expected.OutputPath, cfg.OutputPath)
		})
	}
}

func TestValidate(t *testing.T) {
	t.Run("valid configuration", func(t *testing.T) {
		cfg := &env.GeneralConfig{
			StatePath:  "/state",
			OutputPath: "/output",
		}

		err := cfg.Validate()
		assert.NoError(t, err)
	})

	t.Run("missing state path", func(t *testing.T) {
		cfg := &env.GeneralConfig{
			OutputPath: "/output",
		}

		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "STATE_PATH")
	})

	t.Run("missing output path", func(t *testing.T) {
		cfg := &env.GeneralConfig{
			StatePath: "/state",
		}

		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "OUTPUT_PATH")
	})

	t.Run("both paths missing", func(t *testing.T) {
		cfg := &env.GeneralConfig{}

		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "STATE_PATH or OUTPUT_PATH")
	})

	t.Run("error logging", func(t *testing.T) {
		core, recorded := observer.New(zap.ErrorLevel)
		logger.SetLogger(zap.New(core))
		defer logger.SetLogger(zap.NewNop())

		cfg := &env.GeneralConfig{}
		err := cfg.Validate()

		assert.Error(t, err)
		assert.Equal(t, 1, recorded.Len(), "Should log one error")

		logEntry := recorded.All()[0]
		assert.Equal(t, "Missing ENV credentials: STATE_PATH or OUTPUT_PATH", logEntry.Message)
		assert.Equal(t, zap.ErrorLevel, logEntry.Level)
	})
}
