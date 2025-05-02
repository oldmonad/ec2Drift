package main

import (
	"os"

	"github.com/joho/godotenv"
	"github.com/oldmonad/ec2Drift/internal/app"
	"github.com/oldmonad/ec2Drift/pkg/config/env"
	"github.com/oldmonad/ec2Drift/pkg/errors"
	"github.com/oldmonad/ec2Drift/pkg/logger"
	"github.com/oldmonad/ec2Drift/pkg/ports/cli"
	"github.com/oldmonad/ec2Drift/pkg/ports/rest"
	"github.com/oldmonad/ec2Drift/pkg/utils/validator"
	"go.uber.org/zap"
)

func main() {
	logger.Init(true)
	defer logger.Log.Sync()
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		logger.Log.Error("failed to load .env", zap.Error(err))
		os.Exit(1)
	}

	// Load and parse application configurations from environment variables
	configurations, err := env.SetupConfigurations()
	if err != nil {
		logger.Log.Fatal(errors.NewErrConfigSetup(err).Error(), zap.Error(err))
	}

	// Create core application instance with loaded configurations
	app := app.NewApp(*configurations)

	// Initialize input validator
	validator := validator.NewValidator()

	// Initialize HTTP server that exposes drift detection via REST API
	httpServer := rest.NewServer(app, validator)

	// Prepare CLI command handler with all dependencies injected
	command := cli.NewCommand(app, validator, httpServer, configurations)

	// Construct root command that wires together CLI interface
	rootCmd := command.InitiateCommands()

	// Execute the root command (CLI entrypoint)
	if err := rootCmd.Execute(); err != nil {
		logger.Log.Fatal("command failed", zap.Error(err))
	}
}
