package output

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/oldmonad/ec2Drift/internal/driftchecker"
	"github.com/olekukonko/tablewriter"
)

func PrintTable(reports []driftchecker.DriftReport) {
	red := color.New(color.FgRed).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Instance ID", "Application", "Attribute", "Expected", "Actual"})
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("\t")
	table.SetNoWhiteSpace(true)

	for _, report := range reports {
		for _, drift := range report.Drifts {
			expVal := formatValue(drift.ExpectedValue)
			actVal := formatValue(drift.ActualValue)

			var expColored, actColored string
			if expVal == actVal {
				expColored = green(expVal)
				actColored = green(actVal)
			} else {
				expColored = yellow(expVal)
				actColored = red(actVal)
			}

			table.Append([]string{
				report.InstanceID,
				report.Name,
				drift.Attribute,
				expColored,
				actColored,
			})
		}
	}

	table.Render()
}

func formatValue(v interface{}) string {
	switch val := v.(type) {
	case []string:
		return strings.Join(val, ", ")
	case bool:
		return strconv.FormatBool(val)
	case int:
		return strconv.Itoa(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}
