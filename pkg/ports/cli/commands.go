package cli

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/oldmonad/ec2Drift/internal/app"
	"github.com/oldmonad/ec2Drift/pkg/parser"
	"github.com/oldmonad/ec2Drift/pkg/ports"
	"github.com/oldmonad/ec2Drift/pkg/ports/rest"
	"github.com/spf13/cobra"
)

var ValidAttrs = map[string]bool{
	"instance_type":                 true,
	"security_groups":               true,
	"ami":                           true,
	"tags":                          true,
	"root_block_device.volume_size": true,
	"root_block_device.volume_type": true,
}

var (
	ValidateAttributes = validateAttributes
	ServerStarter      = rest.StartServer
)

func NewCommand(appInstance app.AppRunner) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "ec2drift",
		Short: "Detect drift between configuration and cloud provider",
	}

	var format string
	var attrList []string

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run drift check",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(attrList)
			attrs, err := validateAttributes(attrList)
			fmt.Println(attrs)

			if err != nil {
				return err
			}

			parserType := parser.Terraform
			if format == "json" {
				parserType = parser.JSON
			}

			return appInstance.Run(cmd.Context(), attrs, parserType, ports.CLI)
		},
	}

	var port string
	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			port := cmd.Flag("port").Value.String()
			if _, err := strconv.Atoi(port); err != nil {
				return fmt.Errorf("invalid port %q: must be a numeric value", port)
			}
			if portNum, _ := strconv.Atoi(port); portNum < 1 || portNum > 65535 {
				return fmt.Errorf("port %d out of range [1-65535]", portNum)
			}
			return ServerStarter(appInstance, port)
		},
	}

	serveCmd.Flags().StringVar(&port, "port", "8080", "port for HTTP server")
	runCmd.Flags().StringVar(&format, "format", "terraform", "input format: terraform or json")
	runCmd.Flags().StringSliceVarP(&attrList, "attributes", "a", []string{}, "optional attributes to check for drift (comma-separated or multiple flags)")

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(serveCmd)
	return rootCmd
}

func validateAttributes(requested []string) ([]string, error) {
	if len(requested) == 0 {
		out := make([]string, 0, len(ValidAttrs))
		for k := range ValidAttrs {
			out = append(out, k)
		}
		return out, nil
	}
	for _, a := range requested {
		if !ValidAttrs[a] {
			return nil, fmt.Errorf("invalid attribute %q; valid are: %v", a, keys())
		}
	}
	return requested, nil
}

func keys() []string {
	ks := make([]string, 0, len(ValidAttrs))
	for k := range ValidAttrs {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
