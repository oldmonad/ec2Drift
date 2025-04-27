package cli_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/oldmonad/ec2Drift/internal/app"
	"github.com/oldmonad/ec2Drift/pkg/config/env"
	"github.com/oldmonad/ec2Drift/pkg/parser"
	"github.com/oldmonad/ec2Drift/pkg/ports"
	"github.com/oldmonad/ec2Drift/pkg/ports/cli"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

type mockApp struct {
	lastFormat parser.ParserType
	lastAttrs  []string
}

func (m *mockApp) Run(ctx context.Context, attrs []string, format parser.ParserType, runtype ports.Runtype) error {
	m.lastAttrs = attrs
	m.lastFormat = format
	return nil
}

func TestNewCommand(t *testing.T) {
	appInstance := app.New(env.GeneralConfig{})
	cmd := cli.NewCommand(appInstance)

	subs := cmd.Commands()
	if len(subs) != 2 {
		t.Fatalf("Expected 2 subcommands, got %d", len(subs))
	}

	runCmd := getCommand(cmd, "run")
	if runCmd == nil {
		t.Fatal("'run' subcommand not found")
	}
	checkFlag(t, runCmd, "format")
	checkFlag(t, runCmd, "attributes")

	serveCmd := getCommand(cmd, "serve")
	if serveCmd == nil {
		t.Fatal("'serve' subcommand not found")
	}
	checkFlag(t, serveCmd, "port")
}

func TestServeCommandPortFlag(t *testing.T) {
	appInstance := app.New(env.GeneralConfig{})
	appInstance.Logger = zap.NewNop()

	originalStarter := cli.ServerStarter
	t.Cleanup(func() { cli.ServerStarter = originalStarter })

	var calledWithPort string
	cli.ServerStarter = func(appRunner app.AppRunner, port string) error {
		calledWithPort = port
		return nil
	}

	t.Run("valid_port", func(t *testing.T) {
		calledWithPort = ""

		cmd := cli.NewCommand(appInstance)
		cmd.SetArgs([]string{"serve", "--port=8080"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if calledWithPort != "8080" {
			t.Errorf("Expected port 8080, got %s", calledWithPort)
		}
	})

	t.Run("invalid_port", func(t *testing.T) {
		calledWithPort = ""

		cmd := cli.NewCommand(appInstance)
		cmd.SetArgs([]string{"serve", "--port=invalid"})
		err := cmd.Execute()
		if err == nil {
			t.Fatal("Expected error but got none")
		}
		if calledWithPort != "" {
			t.Error("Server starter should not be called with invalid port")
		}
	})
}

func TestRunCommandFormatFlag(t *testing.T) {
	app := &mockApp{}
	cmd := cli.NewCommand(app)

	tests := []struct {
		format string
		want   parser.ParserType
	}{
		{"terraform", parser.Terraform},
		{"json", parser.JSON},
		{"invalid", parser.Terraform},
		{"", parser.Terraform},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {

			app.lastFormat = ""

			args := []string{"run"}
			if tt.format != "" {
				args = append(args, "--format="+tt.format)
			}

			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("Expected no error for format %q, got: %v", tt.format, err)
			}

			if app.lastFormat != tt.want {
				t.Errorf("For format %q: expected parser type %v, got %v",
					tt.format, tt.want, app.lastFormat)
			}
		})
	}
}

func TestValidateAttributes(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    []string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "no attributes",
			input:   nil,
			want:    []string{"ami", "instance_type", "security_groups", "tags", "root_block_device.volume_size", "root_block_device.volume_type"},
			wantErr: false,
		},
		{
			name:    "valid attributes",
			input:   []string{"ami", "tags"},
			want:    []string{"ami", "tags"},
			wantErr: false,
		},
		{
			name:    "invalid attribute",
			input:   []string{"invalid"},
			wantErr: true,
			errMsg:  "invalid attribute \"invalid\"; valid are: [ami instance_type root_block_device.volume_size root_block_device.volume_type security_groups tags]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cli.ValidateAttributes(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateAttributes() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if err.Error() != tt.errMsg {
					t.Errorf("validateAttributes() error = %v, want %v", err.Error(), tt.errMsg)
				}
				return
			}
			gotSet := make(map[string]struct{})
			for _, v := range got {
				gotSet[v] = struct{}{}
			}
			wantSet := make(map[string]struct{})
			for _, v := range tt.want {
				wantSet[v] = struct{}{}
			}
			if !reflect.DeepEqual(gotSet, wantSet) {
				t.Errorf("validateAttributes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func getCommand(cmd *cobra.Command, name string) *cobra.Command {
	for _, sub := range cmd.Commands() {
		if sub.Name() == name {
			return sub
		}
	}
	return nil
}

func checkFlag(t *testing.T, cmd *cobra.Command, name string) {
	if cmd.Flags().Lookup(name) == nil {
		t.Errorf("Flag %s not found in command %s", name, cmd.Name())
	}
}
