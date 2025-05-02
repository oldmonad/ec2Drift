package cli

import (
	"github.com/oldmonad/ec2Drift/internal/app"
	"github.com/oldmonad/ec2Drift/pkg/config/env"
	"github.com/oldmonad/ec2Drift/pkg/ports"
	"github.com/oldmonad/ec2Drift/pkg/ports/rest"
	validation "github.com/oldmonad/ec2Drift/pkg/utils/validator"
	"github.com/spf13/cobra"
)

// Command encapsulates CLI dependencies and logic
type Command struct {
	app               app.AppRunner        // Core application logic runner
	validator         validation.Validator // Input validator for CLI args
	server            rest.Server          // REST server instance
	envConfigurations env.Config           // Configuration loaded from environment
}

// NewCommand creates a new CLI command handler with injected dependencies
func NewCommand(
	app app.AppRunner,
	validator validation.Validator,
	server rest.Server,
	envConfigurations *env.Configurations,
) *Command {
	return &Command{
		app:               app,
		validator:         validator,
		server:            server,
		envConfigurations: envConfigurations,
	}
}

// InitiateCommands initializes the root command and all CLI subcommands
func (cf *Command) InitiateCommands() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "ec2drift",
		Short: "Detect drift between configuration and cloud provider",
	}

	// Attach "run" and "serve" subcommands to root
	rootCmd.AddCommand(cf.createRunCommand())
	rootCmd.AddCommand(cf.createServeCommand())

	return rootCmd
}

// createRunCommand defines the "run" subcommand which executes drift detection logic
func (cf *Command) createRunCommand() *cobra.Command {
	var format string          // Input format: terraform or json
	var attributeList []string // List of specific attributes to validate

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run drift check",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate and parse input format (e.g., terraform, json)
			parserType, err := cf.validator.ValidateFormat(format)
			if err != nil {
				return err
			}

			// Validate user-provided attribute filters
			validAttributes, err := cf.validator.ValidateAttributes(attributeList)
			if err != nil {
				return err
			}

			// Run the application drift detection logic
			return cf.app.Run(cmd.Context(), validAttributes, parserType, ports.CLI)
		},
	}

	// Register CLI flags
	runCmd.Flags().StringVar(&format, "format", "terraform", "input format: terraform or json")
	runCmd.Flags().StringSliceVarP(&attributeList, "attributes", "a", []string{},
		"optional attributes to check for drift (comma-separated or multiple flags)")

	return runCmd
}

// createServeCommand defines the "serve" subcommand which starts the HTTP server
func (cf *Command) createServeCommand() *cobra.Command {
	var httpPort string // CLI override for HTTP port (optional)

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Start the HTTP server on the configured port
			return cf.server.Start(cf.envConfigurations.PortToString())
		},
	}

	// Register CLI flag to allow port override
	serveCmd.Flags().StringVar(&httpPort, "port", httpPort, "port for HTTP server")

	return serveCmd
}
