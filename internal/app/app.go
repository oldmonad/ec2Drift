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
	Logger             *zap.Logger
	generalCfg         env.GeneralConfig
	fetchInstancesFunc func(ctx context.Context, cfg config.ProviderConfig) ([]cloud.Instance, error)
	parseFunc          func(content []byte) ([]cloud.Instance, error)
}

type AppRunner interface {
	Run(ctx context.Context, attrs []string, format parser.ParserType, runtype ports.Runtype) error
}

func New(generalConfig env.GeneralConfig) *App {
	return &App{Logger: logger.Log, generalCfg: generalConfig}
}

func NewTestable(
	generalConfig env.GeneralConfig,
	fetchInstancesFunc func(ctx context.Context, cfg config.ProviderConfig) ([]cloud.Instance, error),
	parseFunc func(content []byte) ([]cloud.Instance, error),
) *App {
	app := New(generalConfig)
	app.fetchInstancesFunc = fetchInstancesFunc
	app.parseFunc = parseFunc

	return app
}

func (a *App) Run(ctx context.Context, attrs []string, format parser.ParserType, runtype ports.Runtype) error {
	statePath := a.generalCfg.StatePath
	log := a.Logger.With(zap.String("component", "app"))
	log.Info("Reading configuration file", zap.String("path", statePath))
	content, err := os.ReadFile(statePath)

	if err != nil {
		return errors.NewReadFileError(err)
	}

	providerType := config.ProviderType(os.Getenv("CLOUD_PROVIDER"))
	cloudCfg, err := config.NewProviderConfig(providerType)

	if err != nil {
		log.Error("invalid cloud provider", zap.Error(err))
		return errors.ErrInvalidCloudProvider{Provider: string(providerType)}
	}

	if err := cloudCfg.Validate(); err != nil {
		log.Error("cloud config validation failed", zap.Error(err))
		return errors.ErrCloudConfigValidation{Reason: err.Error()}
	}

	var stateInstances []cloud.Instance
	if a.fetchInstancesFunc != nil {
		stateInstances, err = a.fetchInstancesFunc(ctx, cloudCfg)
	} else {
		var provider cloud.CloudProvider
		switch providerType {
		case config.AWS:
			provider = &aws.AWSProvider{}
		case config.GCP:
			provider = &gcp.GCPProvider{}
		default:
			provider = &aws.AWSProvider{}
		}
		stateInstances, err = provider.FetchInstances(ctx, cloudCfg)
	}

	var configInstances []cloud.Instance
	if a.parseFunc != nil {
		configInstances, err = a.parseFunc(content)
	} else {
		var p parser.Parser
		switch format {
		case parser.Terraform:
			p = &parser.TerraformParser{}
		case parser.JSON:
			p = &parser.JSONParser{}
		default:
			p = &parser.TerraformParser{}
		}
		configInstances, err = p.Parse(content)
	}

	if err != nil {
		return errors.ErrParse{Err: err}
	}

	if len(configInstances) == 0 {
		return errors.ErrNoEC2Instances{Path: statePath}
	}

	if len(attrs) == 0 {
		attrs = []string{
			"ami",
			"instance_type",
			"security_groups",
			"tags",
			"root_block_device.volume_size",
			"root_block_device.volume_type",
		}
	}
	reports := driftchecker.Detect(ctx, stateInstances, configInstances, attrs)
	if len(reports) > 0 {
		log.Info("Drift detected", zap.Int("report_count", len(reports)))
		output.PrintTable(reports)
		if runtype == ports.CLI {
			os.Exit(0)
		}

		return errors.NewDriftDetected()
	}

	log.Info("No drift detected")
	return nil
}
