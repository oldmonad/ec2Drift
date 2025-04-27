package env

import (
	"fmt"
	"os"
	"strconv"

	"github.com/oldmonad/ec2Drift/pkg/logger"
)

type GeneralConfig struct {
	DebugMode  bool
	LogLevel   string
	ConfigPath string
	StatePath  string
	OutputPath string
}

func LoadGeneralConfig() *GeneralConfig {
	debug, err := strconv.ParseBool(os.Getenv("DEBUG"))

	if err != nil {
		debug = false
	}

	return &GeneralConfig{
		DebugMode:  debug,
		LogLevel:   os.Getenv("LOG_LEVEL"),
		ConfigPath: os.Getenv("CONFIG_PATH"),
		StatePath:  os.Getenv("STATE_PATH"),
		OutputPath: os.Getenv("OUTPUT_PATH"),
	}
}

func (c *GeneralConfig) Validate() error {
	if c.StatePath == "" || c.OutputPath == "" {
		errMsg := "Missing ENV credentials: STATE_PATH or OUTPUT_PATH"
		logger.Log.Error(errMsg)
		return fmt.Errorf(errMsg)
	}

	return nil
}
