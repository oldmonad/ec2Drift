package app

import (
	"context"
	"os"

	"github.com/oldmonad/ec2Drift/internal/driftchecker"
	"github.com/oldmonad/ec2Drift/pkg/cloud"
	"github.com/oldmonad/ec2Drift/pkg/cloud/aws"
	"github.com/oldmonad/ec2Drift/pkg/cloud/gcp"
	config "github.com/oldmonad/ec2Drift/pkg/config/cloud"
	"github.com/oldmonad/ec2Drift/pkg/config/env"
	"github.com/oldmonad/ec2Drift/pkg/errors"
	"github.com/oldmonad/ec2Drift/pkg/logger"
	"github.com/oldmonad/ec2Drift/pkg/output"
	"github.com/oldmonad/ec2Drift/pkg/parser"
	"github.com/oldmonad/ec2Drift/pkg/ports"
	"go.uber.org/zap"
)

type App struct {
	Logger         *zap.Logger
	configurations env.Configurations
}

// AppRunner defines the contract for running the core application logic
type AppRunner interface {
	Run(ctx context.Context, attrs []string, format parser.ParserType, runtype ports.Runtype) error
}

// NewApp initializes and returns a new App instance
func NewApp(configurations env.Configurations) *App {
	return &App{Logger: logger.Log, configurations: configurations}
}

// Configurations returns the application's configuration settings
func (a *App) Configurations() env.Configurations {
	return a.configurations
}

// Run orchestrates the full drift detection workflow:
// 1. Fetch current cloud state
// 2. Load desired configuration from file
// 3. Parse desired state
// 4. Compare actual vs. desired and report drift
func (a *App) Run(ctx context.Context, attrs []string, format parser.ParserType, runtype ports.Runtype) error {
	stateInstances, err := a.GetLiveStateInstances(ctx, a.configurations.CloudConfig)
	if err != nil {
		return err
	}

	content, err := a.LoadStateFile()
	if err != nil {
		return err
	}

	configInstances, err := a.ParseConfigInstances(content, format)
	if err != nil {
		return err
	}

	return a.HandleDrift(ctx, stateInstances, configInstances, attrs, runtype)
}

// LoadStateFile reads and returns the contents of the desired state configuration file
// if I had more time, I would refactor this to use a more robust file reading mechanism
// which would be part of a separate module that handles file and data operations
func (a *App) LoadStateFile() ([]byte, error) {
	path := a.configurations.StatePath
	a.Logger.Info("Reading configuration file", zap.String("path", path))
	data, err := os.ReadFile(path)
	if err != nil {
		a.Logger.Error("Failed to read configuration file", zap.Error(err))
		return nil, errors.NewReadFileError(err)
	}
	a.Logger.Info("Configuration file read successfully")
	return data, nil
}

// GetLiveStateInstances orchestrates and sets the cloud provider instance data
// And then proceeds to fetch the live state instances from the cloud provider
func (a *App) GetLiveStateInstances(ctx context.Context, configurations config.ProviderConfig) ([]cloud.Instance, error) {
	var provider cloud.CloudProvider
	switch a.configurations.CloudProviderType {
	case config.AWS:
		provider = &aws.AWSProvider{}
	case config.GCP:
		provider = &gcp.GCPProvider{}
	default:
		// Default to AWS if provider is not specified
		provider = &aws.AWSProvider{}
	}
	return provider.FetchInstances(ctx, configurations)
}

// ParseConfigInstances parses the desired configuration content into structured instance data
func (a *App) ParseConfigInstances(content []byte, format parser.ParserType) ([]cloud.Instance, error) {
	var p parser.Parser
	switch format {
	case parser.Terraform:
		p = &parser.TerraformParser{}
	case parser.JSON:
		p = &parser.JSONParser{}
	default:
		// Default to Terraform parser if format is unrecognized
		p = &parser.TerraformParser{}
	}
	return p.Parse(content)
}

// HandleDrift compares actual vs. desired instances and outputs the drift report
func (a *App) HandleDrift(
	ctx context.Context,
	stateInstances, configInstances []cloud.Instance,
	attrs []string,
	runtype ports.Runtype,
) error {
	reports := driftchecker.Detect(ctx, stateInstances, configInstances, attrs)
	if len(reports) > 0 {
		a.Logger.Info("Drift detected", zap.Int("report_count", len(reports)))
		output.PrintTable(reports)

		// In CLI mode, exit after printing drift
		if runtype == ports.CLI {
			os.Exit(0)
		}
		return errors.NewDriftDetected()
	}

	a.Logger.Info("No drift detected")
	return nil
}
