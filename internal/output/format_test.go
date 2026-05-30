package output

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout captures stdout output during function execution
func captureStdout(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// captureStderr captures stderr output during function execution
func captureStderr(f func()) string {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	f()

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestSetFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected Format
	}{
		{"json", FormatJSON},
		{"table", FormatTable},
		{"invalid", FormatTable},
		{"", FormatTable},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			SetFormat(tt.input)
			if CurrentFormat != tt.expected {
				t.Errorf("SetFormat(%q) = %v, want %v", tt.input, CurrentFormat, tt.expected)
			}
		})
	}

	// Reset to default
	CurrentFormat = FormatTable
}

func TestTable(t *testing.T) {
	headers := []string{"NAME", "VALUE"}
	table := Table(headers)

	if table == nil {
		t.Fatal("Table() returned nil")
	}
}

func TestJSON(t *testing.T) {
	data := map[string]string{"key": "value"}

	output := captureStdout(func() {
		err := JSON(data)
		if err != nil {
			t.Errorf("JSON() error: %v", err)
		}
	})

	if !strings.Contains(output, `"key": "value"`) {
		t.Errorf("JSON() output = %q, expected to contain key:value", output)
	}
}

func TestPrint_JSON(t *testing.T) {
	CurrentFormat = FormatJSON
	defer func() { CurrentFormat = FormatTable }()

	headers := []string{"COL1", "COL2"}
	rows := [][]string{{"a", "b"}}
	jsonData := map[string]string{"col1": "a", "col2": "b"}

	output := captureStdout(func() {
		err := Print(headers, rows, jsonData)
		if err != nil {
			t.Errorf("Print() error: %v", err)
		}
	})

	if !strings.Contains(output, "col1") {
		t.Errorf("Print() in JSON mode should output JSON, got: %q", output)
	}
}

func TestPrint_Table(t *testing.T) {
	CurrentFormat = FormatTable

	headers := []string{"NAME", "VALUE"}
	rows := [][]string{{"foo", "bar"}}
	jsonData := map[string]string{}

	output := captureStdout(func() {
		err := Print(headers, rows, jsonData)
		if err != nil {
			t.Errorf("Print() error: %v", err)
		}
	})

	if !strings.Contains(output, "NAME") || !strings.Contains(output, "foo") {
		t.Errorf("Print() table output = %q, expected headers and data", output)
	}
}

func TestPrint_LargeDataset(t *testing.T) {
	CurrentFormat = FormatTable

	headers := []string{"NAME"}
	rows := make([][]string, 60) // > 50 triggers PrintSimple
	for i := 0; i < 60; i++ {
		rows[i] = []string{"item"}
	}

	output := captureStdout(func() {
		err := Print(headers, rows, nil)
		if err != nil {
			t.Errorf("Print() error: %v", err)
		}
	})

	// PrintSimple uses tab separators
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 61 { // header + 60 rows
		t.Errorf("PrintSimple() should output 61 lines, got %d", len(lines))
	}
}

func TestPrintSimple(t *testing.T) {
	headers := []string{"A", "B", "C"}
	rows := [][]string{
		{"1", "2", "3"},
		{"4", "5", "6"},
	}

	output := captureStdout(func() {
		err := PrintSimple(headers, rows)
		if err != nil {
			t.Errorf("PrintSimple() error: %v", err)
		}
	})

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		t.Errorf("PrintSimple() should output 3 lines, got %d", len(lines))
	}

	// Check tab-separated format
	if !strings.Contains(lines[0], "A\tB\tC") {
		t.Errorf("PrintSimple() header = %q, expected tab-separated", lines[0])
	}
}

func TestPrintSingle_JSON(t *testing.T) {
	CurrentFormat = FormatJSON
	defer func() { CurrentFormat = FormatTable }()

	fields := map[string]string{"name": "test"}
	jsonData := map[string]string{"name": "test"}

	output := captureStdout(func() {
		err := PrintSingle(fields, jsonData)
		if err != nil {
			t.Errorf("PrintSingle() error: %v", err)
		}
	})

	if !strings.Contains(output, `"name"`) {
		t.Errorf("PrintSingle() JSON output = %q, expected JSON", output)
	}
}

func TestPrintSingle_Table(t *testing.T) {
	CurrentFormat = FormatTable

	fields := map[string]string{
		"name":  "test",
		"value": "123",
	}

	output := captureStdout(func() {
		err := PrintSingle(fields, nil)
		if err != nil {
			t.Errorf("PrintSingle() error: %v", err)
		}
	})

	if !strings.Contains(output, "name:") || !strings.Contains(output, "test") {
		t.Errorf("PrintSingle() output = %q, expected formatted fields", output)
	}
}

func TestSuccess(t *testing.T) {
	output := captureStdout(func() {
		Success("Operation %s", "completed")
	})

	// Symbol varies by terminal; just check the message is present.
	if !strings.Contains(output, "completed") {
		t.Errorf("Success() output = %q, expected message", output)
	}
}

func TestError(t *testing.T) {
	output := captureStderr(func() {
		Error("Something %s", "failed")
	})

	if !strings.Contains(output, "failed") {
		t.Errorf("Error() output = %q, expected message", output)
	}
}

func TestInfo(t *testing.T) {
	output := captureStdout(func() {
		Info("Info message: %d", 42)
	})

	if !strings.Contains(output, "42") {
		t.Errorf("Info() output = %q, expected message", output)
	}
}

func TestWriter(t *testing.T) {
	w := Writer()
	if w != os.Stdout {
		t.Error("Writer() should return os.Stdout")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		max      int
		expected string
	}{
		{"hello", 10, "hello"},           // no truncation needed
		{"hello", 5, "hello"},            // exact length
		{"hello world", 8, "hello..."},   // truncated with ellipsis
		{"hello", 3, "hel"},              // max <= 3, no ellipsis
		{"hello", 2, "he"},               // max < 3
		{"hello", 0, ""},                 // edge case
		{"", 5, ""},                      // empty string
		{"abcdefghij", 7, "abcd..."},     // normal truncation
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Truncate(tt.input, tt.max)
			if got != tt.expected {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.expected)
			}
		})
	}
}
