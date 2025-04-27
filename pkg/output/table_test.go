package output_test

import (
	"io"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/oldmonad/ec2Drift/internal/driftchecker"
	"github.com/oldmonad/ec2Drift/pkg/output"
	"github.com/stretchr/testify/assert"
)

func init() {
	color.NoColor = false
}

func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf strings.Builder
	io.Copy(&buf, r)
	return buf.String()
}

func TestPrintTableEmptyReports(t *testing.T) {
	output := captureOutput(func() {
		output.PrintTable(nil)
	})

	expectedHeader := "INSTANCE ID\tAPPLICATION\tATTRIBUTE\tEXPECTED\tACTUAL"
	assert.Contains(t, output, expectedHeader)
	assert.True(t, strings.HasPrefix(output, expectedHeader), "Table should start with header")
	assert.Equal(t, 1, strings.Count(output, "\n"), "Only header should be present")
}

func TestPrintTableMatchingValues(t *testing.T) {
	reports := []driftchecker.DriftReport{
		{
			InstanceID: "i-123",
			Name:       "app1",
			Drifts: []driftchecker.DriftDetail{
				{
					Attribute:     "ami",
					ExpectedValue: "ami-123",
					ActualValue:   "ami-123",
				},
			},
		},
	}

	output := captureOutput(func() {
		output.PrintTable(reports)
	})

	pattern := regexp.MustCompile(`i-123\s+app1\s+ami\s+\x1b\[32mami-123\x1b\[0m\s+\x1b\[32mami-123\x1b\[0m`)
	assert.Regexp(t, pattern, output)
}

func TestPrintTableMismatchedValues(t *testing.T) {
	reports := []driftchecker.DriftReport{
		{
			InstanceID: "i-456",
			Name:       "app2",
			Drifts: []driftchecker.DriftDetail{
				{
					Attribute:     "instance_type",
					ExpectedValue: "t2.micro",
					ActualValue:   "t3.micro",
				},
			},
		},
	}

	output := captureOutput(func() {
		output.PrintTable(reports)
	})

	expectedPattern := regexp.MustCompile(`i-456\s+app2\s+instance_type\s+\x1b\[33mt2\.micro\x1b\[0m\s+\x1b\[31mt3\.micro\x1b\[0m`)
	assert.Regexp(t, expectedPattern, output)
}

func TestPrintTableMixedDrifts(t *testing.T) {
	reports := []driftchecker.DriftReport{
		{
			InstanceID: "i-789",
			Name:       "app3",
			Drifts: []driftchecker.DriftDetail{
				{
					Attribute:     "ami",
					ExpectedValue: "ami-match",
					ActualValue:   "ami-match",
				},
				{
					Attribute:     "instance_type",
					ExpectedValue: "t2.medium",
					ActualValue:   "t3.medium",
				},
			},
		},
	}

	output := captureOutput(func() {
		output.PrintTable(reports)
	})

	assert.Contains(t, output, "\x1b[32mami-match\x1b[0m")
	assert.Contains(t, output, "\x1b[33mt2.medium\x1b[0m")
	assert.Contains(t, output, "\x1b[31mt3.medium\x1b[0m")
	assert.True(t, strings.Index(output, "ami") < strings.Index(output, "instance_type"), "AMI should come first")
}

func TestPrintTableFormattingDifferentTypes(t *testing.T) {
	reports := []driftchecker.DriftReport{
		{
			InstanceID: "i-0",
			Name:       "app0",
			Drifts: []driftchecker.DriftDetail{
				{
					Attribute:     "security_groups",
					ExpectedValue: []string{"sg-1", "sg-2"},
					ActualValue:   []string{"sg-3", "sg-4"},
				},
				{
					Attribute:     "ebs_optimized",
					ExpectedValue: true,
					ActualValue:   false,
				},
				{
					Attribute:     "cpu_core_count",
					ExpectedValue: 2,
					ActualValue:   4,
				},
			},
		},
	}

	output := captureOutput(func() {
		output.PrintTable(reports)
	})

	assert.Contains(t, output, "\x1b[33msg-1, sg-2\x1b[0m")
	assert.Contains(t, output, "\x1b[31msg-3, sg-4\x1b[0m")
	assert.Contains(t, output, "\x1b[33mtrue\x1b[0m")
	assert.Contains(t, output, "\x1b[31mfalse\x1b[0m")
	assert.Contains(t, output, "\x1b[33m2\x1b[0m")
	assert.Contains(t, output, "\x1b[31m4\x1b[0m")
}

func TestPrintTableMultipleInstances(t *testing.T) {
	reports := []driftchecker.DriftReport{
		{
			InstanceID: "i-111",
			Name:       "appA",
			Drifts: []driftchecker.DriftDetail{
				{
					Attribute:     "availability_zone",
					ExpectedValue: "us-east-1a",
					ActualValue:   "us-east-1b",
				},
			},
		},
		{
			InstanceID: "i-222",
			Name:       "appB",
			Drifts: []driftchecker.DriftDetail{
				{
					Attribute:     "tags",
					ExpectedValue: map[string]string{"Env": "prod"},
					ActualValue:   map[string]string{"Env": "dev"},
				},
			},
		},
	}

	output := captureOutput(func() {
		output.PrintTable(reports)
	})

	assert.Contains(t, output, "i-111")
	assert.Contains(t, output, "i-222")
	assert.Contains(t, output, "\x1b[33mmap[Env:prod]\x1b[0m")
	assert.Contains(t, output, "\x1b[31mmap[Env:dev]\x1b[0m")
}

func TestPrintTableEdgeCases(t *testing.T) {
	t.Run("empty_string_values", func(t *testing.T) {
		reports := []driftchecker.DriftReport{
			{
				InstanceID: "i-empty",
				Name:       "edgeApp",
				Drifts: []driftchecker.DriftDetail{
					{
						Attribute:     "host_id",
						ExpectedValue: "",
						ActualValue:   "h-123",
					},
				},
			},
		}

		output := captureOutput(func() {
			output.PrintTable(reports)
		})

		assert.Contains(t, output, "\x1b[33m\x1b[0m")
		assert.Contains(t, output, "\x1b[31mh-123\x1b[0m")
	})

	t.Run("zero_values", func(t *testing.T) {
		reports := []driftchecker.DriftReport{
			{
				InstanceID: "i-zero",
				Name:       "zeroApp",
				Drifts: []driftchecker.DriftDetail{
					{
						Attribute:     "threads_per_core",
						ExpectedValue: 0,
						ActualValue:   1,
					},
				},
			},
		}

		output := captureOutput(func() {
			output.PrintTable(reports)
		})

		assert.Contains(t, output, "\x1b[33m0\x1b[0m")
		assert.Contains(t, output, "\x1b[31m1\x1b[0m")
	})
}
