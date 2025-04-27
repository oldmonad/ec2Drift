package main

import (
	"github.com/joho/godotenv"
	"github.com/oldmonad/ec2Drift/internal/app"
	"github.com/oldmonad/ec2Drift/pkg/config/env"
	"github.com/oldmonad/ec2Drift/pkg/logger"
	"github.com/oldmonad/ec2Drift/pkg/ports/cli"
	"go.uber.org/zap"
)

func main() {
	if err := godotenv.Load(); err != nil {
		panic("Error loading .env: " + err.Error())
	}

	generalCfg := env.LoadGeneralConfig()

	logger.Init(generalCfg.DebugMode)
	defer logger.Log.Sync()

	if err := generalCfg.Validate(); err != nil {
		logger.Log.Fatal("invalid configuration", zap.Error(err))
	}

	appInstance := app.New(*generalCfg)

	rootCmd := cli.NewCommand(appInstance)

	if err := rootCmd.Execute(); err != nil {
		logger.Log.Fatal("command failed", zap.Error(err))
	}
}
