package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
)

// Format represents the output format
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
)

// CurrentFormat holds the global output format setting
var CurrentFormat Format = FormatTable

// SetFormat sets the global output format
func SetFormat(f string) {
	switch f {
	case "json":
		CurrentFormat = FormatJSON
	default:
		CurrentFormat = FormatTable
	}
}

// Table creates a new table writer with standard styling
func Table(headers []string) *tablewriter.Table {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(headers)
	table.SetBorder(false)
	table.SetHeaderLine(false)
	table.SetColumnSeparator("  ")
	table.SetNoWhiteSpace(true)
	table.SetTablePadding("  ")
	return table
}

// JSON outputs data as JSON
func JSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// Print outputs data in the current format
func Print(headers []string, rows [][]string, jsonData interface{}) error {
	if CurrentFormat == FormatJSON {
		return JSON(jsonData)
	}

	// For large result sets, use simple tab-separated output (faster)
	if len(rows) > 50 {
		return PrintSimple(headers, rows)
	}

	table := Table(headers)
	table.AppendBulk(rows)
	table.Render()
	return nil
}

// PrintSimple prints rows in a simple tab-separated format (fast for large datasets)
func PrintSimple(headers []string, rows [][]string) error {
	// Print header
	fmt.Println(strings.Join(headers, "\t"))

	// Print rows
	for _, row := range rows {
		fmt.Println(strings.Join(row, "\t"))
	}
	return nil
}

// PrintSingle prints a single item's details
func PrintSingle(fields map[string]string, jsonData interface{}) error {
	if CurrentFormat == FormatJSON {
		return JSON(jsonData)
	}

	maxKeyLen := 0
	for key := range fields {
		if len(key) > maxKeyLen {
			maxKeyLen = len(key)
		}
	}

	for key, value := range fields {
		fmt.Printf("%-*s  %s\n", maxKeyLen, key+":", value)
	}
	return nil
}

// Success prints a success message
func Success(format string, args ...interface{}) {
	fmt.Printf("✓ "+format+"\n", args...)
}

// Error prints an error message
func Error(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "✗ "+format+"\n", args...)
}

// Info prints an info message
func Info(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

// Writer returns the appropriate writer
func Writer() io.Writer {
	return os.Stdout
}

// Truncate truncates a string to a maximum length
func Truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
