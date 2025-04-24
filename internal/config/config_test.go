package config_test

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oldmonad/ec2Drift.git/internal/config"
	"github.com/oldmonad/ec2Drift.git/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper: Resets command line flags between tests
func resetFlags() {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
}

// Helper: Creates a temporary file with given content
func createTempFile(t *testing.T, content string) string {
	file, err := os.CreateTemp("", "test-*.tf")
	require.NoError(t, err)
	_, err = file.WriteString(content)
	require.NoError(t, err)
	file.Close()
	return file.Name()
}

func TestNewFromFlagsMissingRequiredFlags(t *testing.T) {
	resetFlags()
	os.Args = []string{"cmd"}

	_, err := config.NewFromFlags()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "both --old-state and --new-state are required")
}

func TestNewFromFlagsWithAttributes(t *testing.T) {
	resetFlags()
	os.Args = []string{
		"cmd",
		"--old-state=old.tfstate",
		"--new-state=new.tfvars",
		"--attributes=ami,id,instance_type",
	}

	cfg, err := config.NewFromFlags()
	require.NoError(t, err)
	assert.Equal(t, "old.tfstate", cfg.OldStatePath)
	assert.Equal(t, "new.tfvars", cfg.NewStatePath)
	assert.Equal(t, []string{"ami", "id", "instance_type"}, cfg.Attributes)
}

func TestNewFromFlagsWithoutAttributes(t *testing.T) {
	resetFlags()
	os.Args = []string{
		"cmd",
		"--old-state=old.tfstate",
		"--new-state=new.tf",
	}

	cfg, err := config.NewFromFlags()
	require.NoError(t, err)
	assert.Greater(t, len(cfg.Attributes), 0)
	assert.Contains(t, cfg.Attributes, "ami")
}

func TestValidateFileInvalidPath(t *testing.T) {
	err := config.ValidateFile("/invalid/path/<>")
	assert.Error(t, err)
}

func TestForNonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "notexist.tf")
	err := config.ValidateFile(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file does not exist")
}

func TestValidateFilePathIsDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	err := config.ValidateFile(tmpDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path is a directory")
}

func TestValidateFileValidFile(t *testing.T) {
	tmpFile := createTempFile(t, "some content")
	defer os.Remove(tmpFile)

	err := config.ValidateFile(tmpFile)
	assert.NoError(t, err)
}

func TestReadStateFilesSuccess(t *testing.T) {
	tmpOld := createTempFile(t, "old content")
	tmpNew := createTempFile(t, "new content")

	cfg := &config.Config{
		OldStatePath: tmpOld,
		NewStatePath: tmpNew,
	}

	err := cfg.ReadStateFiles()
	require.NoError(t, err)
	assert.Equal(t, []byte("old content"), cfg.OldStateContent)
	assert.Equal(t, []byte("new content"), cfg.NewStateContent)
}

func TestReadStateFilesReadError(t *testing.T) {
	cfg := &config.Config{
		OldStatePath: "/nonexistent",
		NewStatePath: "/nonexistent2",
	}

	err := cfg.ReadStateFiles()
	assert.Error(t, err)
}

func TestErrNoEC2InstancesError(t *testing.T) {
	err := config.ErrNoEC2Instances{Path: "/some/path/state.tf"}
	assert.Equal(t, "no AWS EC2 instances found in state file: state.tf", err.Error())
}

func TestReadStateFilesNewReadError(t *testing.T) {
	oldFile := createTempFile(t, "old content")
	defer os.Remove(oldFile)

	tmpDir := t.TempDir()

	cfg := &config.Config{
		OldStatePath: oldFile,
		NewStatePath: tmpDir,
	}

	err := cfg.ReadStateFiles()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error reading new state file:")
}

func TestValidateFileInvalidPathError(t *testing.T) {
	origAbs := config.Abs
	defer func() { config.Abs = origAbs }()
	config.Abs = func(string) (string, error) {
		return "", fmt.Errorf("whoops")
	}

	err := config.ValidateFile("anything")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid path: whoops")
}

func TestLoadAndValidateOldStateFileError(t *testing.T) {
	newFile := createTempFile(t, "new content")
	defer os.Remove(newFile)

	badOld := filepath.Join(t.TempDir(), "does-not-exist.tf")

	cfg := &config.Config{
		OldStatePath: badOld,
		NewStatePath: newFile,
	}

	err := cfg.LoadAndValidate()
	require.Error(t, err)

	assert.Contains(t, err.Error(), "old state file:")
}

func TestLoadAndValidateNewStateFileError(t *testing.T) {
	oldFile := createTempFile(t, "old content")
	defer os.Remove(oldFile)

	badNew := filepath.Join(t.TempDir(), "does-not-exist.tf")

	cfg := &config.Config{
		OldStatePath: oldFile,
		NewStatePath: badNew,
	}

	err := cfg.LoadAndValidate()
	require.Error(t, err)

	assert.Contains(t, err.Error(), "new state file:")
}

func TestLoadAndValidateReadStateFilesError(t *testing.T) {
	newFile := createTempFile(t, "new content")
	defer os.Remove(newFile)

	oldFile := createTempFile(t, "old content")
	defer os.Remove(oldFile)
	require.NoError(t, os.Chmod(oldFile, 0))

	cfg := &config.Config{
		OldStatePath: oldFile,
		NewStatePath: newFile,
	}

	err := cfg.LoadAndValidate()
	require.Error(t, err, "expected LoadAndValidate to fail when ReadStateFiles cannot read old state")

	assert.Contains(t, err.Error(), "error reading state files:")
}

func TestLoadAndValidateStateParseError(t *testing.T) {

	dir := t.TempDir()

	oldState := filepath.Join(dir, "old.tfstate")
	newState := filepath.Join(dir, "new.tfconfig")

	if err := os.WriteFile(oldState, []byte("{ invalid json }"), 0644); err != nil {
		t.Fatalf("failed to write old state file: %v", err)
	}

	if err := os.WriteFile(newState, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to write new state file: %v", err)
	}

	cfg := &config.Config{
		OldStatePath: oldState,
		NewStatePath: newState,
	}

	err := cfg.LoadAndValidate()
	if err == nil {
		t.Fatal("expected an error but got nil")
	}

	if !strings.Contains(err.Error(), "state file parse error") {
		t.Errorf("expected wrapped parse error, got: %v", err)
	}
}

func TestLoadAndValidateNoEC2Instances(t *testing.T) {
	dir := t.TempDir()

	oldState := filepath.Join(dir, "old.tfstate")
	newState := filepath.Join(dir, "new.tfconfig")

	tfstate := `{"version":4,"terraform_version":"1.0.0","resources":[]}`
	if err := os.WriteFile(oldState, []byte(tfstate), 0644); err != nil {
		t.Fatalf("failed to write old state file: %v", err)
	}

	if err := os.WriteFile(newState, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to write new state file: %v", err)
	}

	cfg := &config.Config{
		OldStatePath: oldState,
		NewStatePath: newState,
	}

	err := cfg.LoadAndValidate()
	if err == nil {
		t.Fatal("expected ErrNoEC2Instances, got nil")
	}

	var noEC2Err config.ErrNoEC2Instances
	if !errors.As(err, &noEC2Err) {
		t.Fatalf("expected ErrNoEC2Instances type, got %T: %v", err, err)
	}

	if noEC2Err.Path != oldState {
		t.Errorf("expected error Path %q, got %q", oldState, noEC2Err.Path)
	}
}

func TestLoadAndValidateTFConfigParseError(t *testing.T) {
	dir := t.TempDir()
	oldState := filepath.Join(dir, "old.tfstate")
	newConfig := filepath.Join(dir, "new.tfconfig")

	validState := `{
        "version":4,
        "terraform_version":"1.0.0",
        "resources":[{"type":"aws_instance","instances":[{}]}]
    }`
	if err := os.WriteFile(oldState, []byte(validState), 0644); err != nil {
		t.Fatalf("failed to write old state file: %v", err)
	}

	if err := os.WriteFile(newConfig, []byte("invalid hcl @@"), 0644); err != nil {
		t.Fatalf("failed to write new config file: %v", err)
	}

	cfg := &config.Config{
		OldStatePath: oldState,
		NewStatePath: newConfig,
	}

	err := cfg.LoadAndValidate()
	if err == nil {
		t.Fatal("expected tfconfig parse error, but got nil")
	}

	if !strings.HasPrefix(err.Error(), "tfconfig parse error:") {
		t.Errorf("wrong error prefix\ngot:  %q\nwant: \"tfconfig parse error: ...\"", err.Error())
	}

	underlying := errors.Unwrap(err)
	if underlying == nil {
		t.Error("expected an underlying parse error, but errors.Unwrap returned nil")
	}
}

type mockTerraformState struct{}

func (m *mockTerraformState) HasEC2Instances() bool {
	return true
}

func TestLoadAndValidateSuccess(t *testing.T) {
	oldFile := createTempFile(t, `{"resources":[{"type":"aws_instance"}]}`)
	newFile := createTempFile(t, `{"some":"config"}`)

	cfg := &config.Config{
		OldStatePath: oldFile,
		NewStatePath: newFile,
	}

	cfg.OldStateContent = []byte(`{}`)
	cfg.NewStateContent = []byte(`{}`)

	origParseState := parser.ParseTerraformState
	origParseConfig := parser.ParseTerraformConfig
	defer func() {
		parser.ParseTerraformState = origParseState
		parser.ParseTerraformConfig = origParseConfig
	}()

	parser.ParseTerraformState = func(data []byte) (*parser.TerraformStateFile, error) {
		return &parser.TerraformStateFile{
			Resources: []parser.Resource{
				{Type: "aws_instance", Instances: nil},
			},
		}, nil
	}
	parser.ParseTerraformConfig = func(data []byte) (*parser.TerraformConfig, error) {
		return &parser.TerraformConfig{}, nil
	}

	err := cfg.LoadAndValidate()
	require.NoError(t, err)

	assert.NotNil(t, cfg.TerraformState)
	assert.NotNil(t, cfg.TerraformConfig)
}
