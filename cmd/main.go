package main

import (
	"errors"
	"log"
	"os"

	"github.com/oldmonad/ec2Drift.git/internal/comparator"
	"github.com/oldmonad/ec2Drift.git/internal/config"
	"github.com/oldmonad/ec2Drift.git/internal/output"
)

func main() {
	cfg, err := config.NewFromFlags()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	if err := cfg.LoadAndValidate(); err != nil {
		var missingErr config.ErrNoEC2Instances
		if errors.As(err, &missingErr) {
			log.Fatalf("Validation failed: %v", err)
		}
		log.Fatalf("Configuration error: %v", err)
	}

	// Get instances and configurations
	stateInstances := cfg.TerraformState.GetEC2Instances()
	configs := cfg.TerraformConfig.GetConfigs()

	// Detect drifts
	reports := comparator.DetectDrift(stateInstances, configs, cfg.Attributes)

	// Print results
	if len(reports) > 0 {
		output.PrintTable(reports)
		os.Exit(1)
	}

	log.Println("No drifts detected")
	os.Exit(0)
}
