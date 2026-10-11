package config

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

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
	// *_command values are shell commands whose stdout supplies the credential,
	// so a secret can stay in an external store (e.g. sops) instead of plaintext
	// on disk, and bb can obtain it in any context without a shell wrapper.
	ClientIDCommand     string `mapstructure:"client_id_command"`
	ClientSecretCommand string `mapstructure:"client_secret_command"`
	Workspace           string `mapstructure:"workspace"`
	OutputFormat        string `mapstructure:"output_format"`
	Scopes              string `mapstructure:"scopes"`
	// TokenStore selects how OAuth tokens are persisted: "auto" (default —
	// keyring plus a 0600 file mirror so a dropped keyring entry can't log you
	// out), "keyring" (keyring only), or "file" (0600 file only).
	TokenStore string `mapstructure:"token_store"`
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

// runCredentialCommand runs a shell command and returns its trimmed stdout, or
// "" on empty command or failure. Failures are reported without including the
// command or its output, which may contain a secret. Used to fetch a credential from an external
// store (e.g. `sops -d ...`) so it need not be plaintext on disk.
func runCredentialCommand(command string) string {
	if strings.TrimSpace(command) == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name, args := credentialShell(runtime.GOOS, command)
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = time.Second
	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "bb: credential command failed: %v\n", err)
		return ""
	}
	return strings.TrimSpace(string(out))
}

// credentialShell uses the native command interpreter on each platform.
func credentialShell(goos, command string) (string, []string) {
	if goos == "windows" {
		return "cmd.exe", []string{"/D", "/S", "/C", command}
	}
	return "sh", []string{"-c", command}
}

// GetClientID returns the OAuth client ID. Resolution order:
// BB_CLIENT_ID > BB_CLIENT_ID_COMMAND > config client_id > config client_id_command.
func GetClientID() string {
	if id := os.Getenv("BB_CLIENT_ID"); id != "" {
		return id
	}
	if id := runCredentialCommand(os.Getenv("BB_CLIENT_ID_COMMAND")); id != "" {
		return id
	}
	if id := Get().ClientID; id != "" {
		return id
	}
	return runCredentialCommand(Get().ClientIDCommand)
}

// GetClientSecret returns the OAuth client secret. Resolution order:
// BB_CLIENT_SECRET > BB_CLIENT_SECRET_COMMAND > config client_secret > config client_secret_command.
func GetClientSecret() string {
	if secret := os.Getenv("BB_CLIENT_SECRET"); secret != "" {
		return secret
	}
	if secret := runCredentialCommand(os.Getenv("BB_CLIENT_SECRET_COMMAND")); secret != "" {
		return secret
	}
	if secret := Get().ClientSecret; secret != "" {
		return secret
	}
	return runCredentialCommand(Get().ClientSecretCommand)
}

// GetWorkspace returns the default workspace
func GetWorkspace() string {
	if ws := os.Getenv("BB_WORKSPACE"); ws != "" {
		return ws
	}
	return Get().Workspace
}

// GetTokenStore returns the configured token storage mode ("auto", "keyring", or
// "file"). BB_TOKEN_STORE env overrides the config file; empty means auto.
func GetTokenStore() string {
	if v := os.Getenv("BB_TOKEN_STORE"); v != "" {
		return v
	}
	return Get().TokenStore
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
