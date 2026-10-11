package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/olekukonko/tablewriter"
)

// Format represents the output format
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatCSV   Format = "csv"
)

// CurrentFormat holds the global output format setting
var CurrentFormat Format = FormatTable

// SetFormat sets the global output format
func SetFormat(f string) {
	switch f {
	case "json":
		CurrentFormat = FormatJSON
	case "csv":
		CurrentFormat = FormatCSV
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
	// Empty lists have a stable array shape, including nil typed slices.
	value := reflect.ValueOf(data)
	if value.IsValid() && value.Kind() == reflect.Slice && value.IsNil() {
		data = []interface{}{}
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// Print outputs data in the current format
func Print(headers []string, rows [][]string, jsonData interface{}) error {
	switch CurrentFormat {
	case FormatJSON:
		return JSON(jsonData)
	case FormatCSV:
		w := csv.NewWriter(os.Stdout)
		if len(headers) > 0 {
			if err := w.Write(headers); err != nil {
				return err
			}
		}
		if err := w.WriteAll(rows); err != nil {
			return err
		}
		w.Flush()
		return w.Error()
	}
	table := Table(headers)
	table.AppendBulk(rows)
	table.Render()
	return nil
}

// PrintSimple prints rows in a simple tab-separated format
func PrintSimple(headers []string, rows [][]string) error {
	fmt.Println(strings.Join(headers, "\t"))
	for _, row := range rows {
		fmt.Println(strings.Join(row, "\t"))
	}
	return nil
}

// PrintSingle prints a single item's details in a key:value layout.
// Keys are sorted alphabetically for deterministic output.
func PrintSingle(fields map[string]string, jsonData interface{}) error {
	if CurrentFormat == FormatJSON {
		return JSON(jsonData)
	}

	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	maxKeyLen := 0
	for _, key := range keys {
		if len(key) > maxKeyLen {
			maxKeyLen = len(key)
		}
	}

	for _, key := range keys {
		fmt.Printf("%-*s  %s\n", maxKeyLen, key+":", fields[key])
	}
	return nil
}

// isTerminal reports whether stdout is connected to a terminal.
var isTerminal = func() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// isStderrTerminal reports whether stderr is connected to a terminal.
var isStderrTerminal = func() bool {
	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// symbol returns a plain ASCII fallback when stdout is not a terminal
// (e.g. Windows cmd.exe, CI pipelines, redirected output).
func symbol(unicode, ascii string) string {
	if isTerminal() {
		return unicode
	}
	return ascii
}

// symbolStderr returns a plain ASCII fallback when stderr is not a terminal.
func symbolStderr(unicode, ascii string) string {
	if isStderrTerminal() {
		return unicode
	}
	return ascii
}

// Success prints a success message
func Success(format string, args ...interface{}) {
	fmt.Printf(symbol("✓", "OK")+" "+format+"\n", args...)
}

// Error prints an error message
func Error(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, symbolStderr("✗", "ERROR")+" "+format+"\n", args...)
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
