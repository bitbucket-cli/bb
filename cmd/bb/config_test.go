package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestRunConfigList(t *testing.T) {
	// Save original environment
	origWorkspace := os.Getenv("BB_WORKSPACE")
	origClientID := os.Getenv("BB_CLIENT_ID")
	origClientSecret := os.Getenv("BB_CLIENT_SECRET")
	defer func() {
		os.Setenv("BB_WORKSPACE", origWorkspace)
		os.Setenv("BB_CLIENT_ID", origClientID)
		os.Setenv("BB_CLIENT_SECRET", origClientSecret)
	}()

	// Clear environment
	os.Unsetenv("BB_WORKSPACE")
	os.Unsetenv("BB_CLIENT_ID")
	os.Unsetenv("BB_CLIENT_SECRET")

	tests := []struct {
		name       string
		jsonOutput bool
		setup      func()
		wantErr    bool
	}{
		{
			name:       "Table output",
			jsonOutput: false,
			setup:      func() {},
			wantErr:    false,
		},
		{
			name:       "JSON output",
			jsonOutput: true,
			setup:      func() {},
			wantErr:    false,
		},
		{
			name:       "With env vars",
			jsonOutput: false,
			setup: func() {
				os.Setenv("BB_WORKSPACE", "testworkspace")
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset state
			os.Unsetenv("BB_WORKSPACE")
			os.Unsetenv("BB_CLIENT_ID")
			os.Unsetenv("BB_CLIENT_SECRET")

			jsonOutput = tt.jsonOutput
			tt.setup()

			err := runConfigList(nil, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("runConfigList() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunConfigGet(t *testing.T) {
	// Save original viper state
	origWorkspace := viper.GetString("workspace")
	origOutputFormat := viper.GetString("output_format")
	defer func() {
		viper.Set("workspace", origWorkspace)
		viper.Set("output_format", origOutputFormat)
	}()

	tests := []struct {
		name    string
		args    []string
		setup   func()
		wantErr bool
	}{
		{
			name:    "Get workspace",
			args:    []string{"workspace"},
			setup:   func() { viper.Set("workspace", "myworkspace") },
			wantErr: false,
		},
		{
			name:    "Get output_format",
			args:    []string{"output_format"},
			setup:   func() {},
			wantErr: false,
		},
		{
			name:    "Get client_id",
			args:    []string{"client_id"},
			setup:   func() {},
			wantErr: false,
		},
		{
			name:    "Get client_secret",
			args:    []string{"client_secret"},
			setup:   func() {},
			wantErr: false,
		},
		{
			name:    "Unknown key",
			args:    []string{"unknown_key"},
			setup:   func() {},
			wantErr: true,
		},
		{
			name:    "Case insensitive",
			args:    []string{"WORKSPACE"},
			setup:   func() {},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()

			err := runConfigGet(nil, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("runConfigGet() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunConfigSet(t *testing.T) {
	// Create a temp dir for config
	tmpDir, err := os.MkdirTemp("", "bb-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set up viper to use temp dir
	viper.Reset()
	viper.SetConfigType("yaml")
	viper.SetConfigFile(filepath.Join(tmpDir, "config.yaml"))

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "Set workspace",
			args:    []string{"workspace", "myworkspace"},
			wantErr: false,
		},
		{
			name:    "Set output_format table",
			args:    []string{"output_format", "table"},
			wantErr: false,
		},
		{
			name:    "Set output_format json",
			args:    []string{"output_format", "json"},
			wantErr: false,
		},
		{
			name:    "Set invalid output_format",
			args:    []string{"output_format", "xml"},
			wantErr: true,
		},
		{
			name:    "Set client_id",
			args:    []string{"client_id", "my-client-id"},
			wantErr: false,
		},
		{
			name:    "Set client_secret",
			args:    []string{"client_secret", "my-secret"},
			wantErr: false,
		},
		{
			name:    "Unknown key",
			args:    []string{"unknown_key", "value"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runConfigSet(nil, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("runConfigSet() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunConfigPath(t *testing.T) {
	err := runConfigPath(nil, nil)
	if err != nil {
		t.Errorf("runConfigPath() error = %v", err)
	}
}

func TestMaskIfNotEmpty(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Short string",
			input:    "abc",
			expected: "****",
		},
		{
			name:     "Exactly 4 chars",
			input:    "abcd",
			expected: "****",
		},
		{
			name:     "Longer string",
			input:    "abcdefgh",
			expected: "abcd****",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskIfNotEmpty(tt.input)
			if result != tt.expected {
				t.Errorf("maskIfNotEmpty(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Any value",
			input:    "my-secret-value",
			expected: "********",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskSecret(tt.input)
			if result != tt.expected {
				t.Errorf("maskSecret(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMaskIfSecret(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		expected string
	}{
		{
			name:     "Client secret",
			key:      "client_secret",
			value:    "my-secret",
			expected: "********",
		},
		{
			name:     "Other key",
			key:      "workspace",
			value:    "myworkspace",
			expected: "myworkspace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskIfSecret(tt.key, tt.value)
			if result != tt.expected {
				t.Errorf("maskIfSecret(%q, %q) = %q, want %q", tt.key, tt.value, result, tt.expected)
			}
		})
	}
}

func TestGetSource(t *testing.T) {
	// Save and restore environment
	origValue := os.Getenv("BB_WORKSPACE")
	defer os.Setenv("BB_WORKSPACE", origValue)

	tests := []struct {
		name     string
		key      string
		envVar   string
		setup    func()
		expected string
	}{
		{
			name:   "From environment",
			key:    "workspace",
			envVar: "BB_WORKSPACE",
			setup: func() {
				os.Setenv("BB_WORKSPACE", "envvalue")
			},
			expected: "env",
		},
		{
			name:   "From config",
			key:    "workspace",
			envVar: "BB_WORKSPACE",
			setup: func() {
				os.Unsetenv("BB_WORKSPACE")
				viper.Set("workspace", "configvalue")
			},
			expected: "config",
		},
		{
			name:   "Default",
			key:    "nonexistent",
			envVar: "",
			setup: func() {
				os.Unsetenv("BB_WORKSPACE")
			},
			expected: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			result := getSource(tt.key, tt.envVar)
			if result != tt.expected {
				t.Errorf("getSource(%q, %q) = %q, want %q", tt.key, tt.envVar, result, tt.expected)
			}
		})
	}
}

func TestGetValueWithSource(t *testing.T) {
	// Save and restore environment
	origValue := os.Getenv("BB_WORKSPACE")
	defer os.Setenv("BB_WORKSPACE", origValue)

	tests := []struct {
		name        string
		key         string
		configValue string
		envVar      string
		setup       func()
		expected    string
	}{
		{
			name:        "From environment",
			key:         "workspace",
			configValue: "configvalue",
			envVar:      "BB_WORKSPACE",
			setup: func() {
				os.Setenv("BB_WORKSPACE", "envvalue")
			},
			expected: "envvalue",
		},
		{
			name:        "From config when no env",
			key:         "workspace",
			configValue: "configvalue",
			envVar:      "BB_WORKSPACE",
			setup: func() {
				os.Unsetenv("BB_WORKSPACE")
			},
			expected: "configvalue",
		},
		{
			name:        "Empty when no env and no config",
			key:         "workspace",
			configValue: "",
			envVar:      "BB_WORKSPACE",
			setup: func() {
				os.Unsetenv("BB_WORKSPACE")
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			result := getValueWithSource(tt.key, tt.configValue, tt.envVar)
			if result != tt.expected {
				t.Errorf("getValueWithSource(%q, %q, %q) = %q, want %q",
					tt.key, tt.configValue, tt.envVar, result, tt.expected)
			}
		})
	}
}
