package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

const (
	AppName       = "bb"
	ConfigDirName = "bb"
)

// Config holds all configuration values
type Config struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	Workspace    string `mapstructure:"workspace"`
	OutputFormat string `mapstructure:"output_format"`
	Scopes       string `mapstructure:"scopes"`
}

var cfg *Config

// homeDirFunc allows overriding os.UserHomeDir for testing
var homeDirFunc = os.UserHomeDir

// configDir returns the configuration directory path
func configDir() (string, error) {
	home, err := homeDirFunc()
	if err != nil {
		return "", fmt.Errorf("could not find home directory: %w", err)
	}
	return filepath.Join(home, ".config", ConfigDirName), nil
}

// ConfigDir returns the configuration directory path (exported)
func ConfigDir() (string, error) {
	return configDir()
}

// Init initializes the configuration system
func Init() error {
	dir, err := configDir()
	if err != nil {
		return err
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("could not create config directory: %w", err)
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(dir)

	// Set defaults
	viper.SetDefault("output_format", "table")

	// Environment variable bindings (BB_CLIENT_ID, BB_CLIENT_SECRET, etc.)
	viper.SetEnvPrefix("BB")
	viper.AutomaticEnv()

	// Read config file (ignore if not found)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("error reading config: %w", err)
		}
	}

	cfg = &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		return fmt.Errorf("error parsing config: %w", err)
	}

	return nil
}

// Get returns the current configuration
func Get() *Config {
	if cfg == nil {
		return &Config{}
	}
	return cfg
}

// GetClientID returns the OAuth client ID from env or config
func GetClientID() string {
	// Env vars take precedence
	if id := os.Getenv("BB_CLIENT_ID"); id != "" {
		return id
	}
	return Get().ClientID
}

// GetClientSecret returns the OAuth client secret from env or config
func GetClientSecret() string {
	if secret := os.Getenv("BB_CLIENT_SECRET"); secret != "" {
		return secret
	}
	return Get().ClientSecret
}

// GetWorkspace returns the default workspace
func GetWorkspace() string {
	if ws := os.Getenv("BB_WORKSPACE"); ws != "" {
		return ws
	}
	return Get().Workspace
}

// GetScopes returns the configured OAuth scope level or explicit list
// (env BB_SCOPES takes precedence over the config file). Empty means "use the
// default level".
func GetScopes() string {
	if s := os.Getenv("BB_SCOPES"); s != "" {
		return s
	}
	return Get().Scopes
}

// SetScopes saves the OAuth scope level/list to the config file.
func SetScopes(scopes string) error {
	viper.Set("scopes", scopes)
	return writeConfig()
}

// SetClientCredentials saves OAuth client credentials to config file
func SetClientCredentials(clientID, clientSecret string) error {
	viper.Set("client_id", clientID)
	viper.Set("client_secret", clientSecret)
	return writeConfig()
}

// SetWorkspace saves the default workspace to config file
func SetWorkspace(workspace string) error {
	viper.Set("workspace", workspace)
	return writeConfig()
}

func writeConfig() error {
	dir, err := configDir()
	if err != nil {
		return err
	}

	configPath := filepath.Join(dir, "config.yaml")
	if err := viper.WriteConfigAs(configPath); err != nil {
		return err
	}
	// The file can hold the OAuth client secret — keep it owner-only rather than
	// the 0644 viper writes by default.
	if err := os.Chmod(configPath, 0600); err != nil {
		return fmt.Errorf("could not secure config file permissions: %w", err)
	}
	return nil
}

// HasClientCredentials checks if OAuth credentials are configured
func HasClientCredentials() bool {
	return GetClientID() != "" && GetClientSecret() != ""
}
