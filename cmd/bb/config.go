package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/ThomasRedstone/bb/internal/config"
	"github.com/ThomasRedstone/bb/internal/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
	Long: `View and modify CLI configuration settings.

Configuration is stored in ~/.config/bb/config.yaml

Environment variables (BB_*) take precedence over config file values.`,
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configuration settings",
	Long:  `Display all configuration settings and their current values.`,
	Args:  cobra.NoArgs,
	RunE:  runConfigList,
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Long: `Get the value of a specific configuration setting.

Available keys:
  workspace      - Default workspace for commands
  output_format  - Default output format (table, json)
  client_id      - OAuth client ID
  client_secret  - OAuth client secret (masked)`,
	Args: cobra.ExactArgs(1),
	RunE: runConfigGet,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long: `Set a configuration value.

Available keys:
  workspace      - Default workspace for commands
  output_format  - Default output format (table, json)
  client_id      - OAuth client ID
  client_secret  - OAuth client secret

Example:
  bb config set workspace myworkspace
  bb config set output_format json`,
	Args: cobra.ExactArgs(2),
	RunE: runConfigSet,
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show configuration file path",
	Long:  `Display the path to the configuration file.`,
	Args:  cobra.NoArgs,
	RunE:  runConfigPath,
}

func init() {
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configPathCmd)
}

func runConfigList(cmd *cobra.Command, args []string) error {
	cfg := config.Get()

	// Show all settings with source info
	settings := []struct {
		key    string
		value  string
		source string
	}{
		{
			key:    "workspace",
			value:  getValueWithSource("workspace", cfg.Workspace, "BB_WORKSPACE"),
			source: getSource("workspace", "BB_WORKSPACE"),
		},
		{
			key:    "output_format",
			value:  getValueWithSource("output_format", cfg.OutputFormat, ""),
			source: getSource("output_format", ""),
		},
		{
			key:    "client_id",
			value:  maskIfNotEmpty(config.GetClientID()),
			source: getSource("client_id", "BB_CLIENT_ID"),
		},
		{
			key:    "client_secret",
			value:  maskSecret(config.GetClientSecret()),
			source: getSource("client_secret", "BB_CLIENT_SECRET"),
		},
	}

	if jsonOutput {
		jsonData := make(map[string]interface{})
		for _, s := range settings {
			jsonData[s.key] = map[string]interface{}{
				"value":  s.value,
				"source": s.source,
			}
		}
		return output.JSON(jsonData)
	}

	headers := []string{"KEY", "VALUE", "SOURCE"}
	var rows [][]string
	for _, s := range settings {
		rows = append(rows, []string{s.key, s.value, s.source})
	}

	return output.Print(headers, rows, nil)
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	key := strings.ToLower(args[0])

	var value string
	switch key {
	case "workspace":
		value = config.GetWorkspace()
	case "output_format":
		value = config.Get().OutputFormat
		if value == "" {
			value = "table"
		}
	case "client_id":
		value = config.GetClientID()
	case "client_secret":
		value = maskSecret(config.GetClientSecret())
	default:
		return fmt.Errorf("unknown configuration key: %s", key)
	}

	if value == "" {
		output.Info("Key '%s' is not set", key)
		return nil
	}

	fmt.Println(value)
	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := strings.ToLower(args[0])
	value := args[1]

	switch key {
	case "workspace":
		if err := config.SetWorkspace(value); err != nil {
			return fmt.Errorf("failed to set workspace: %w", err)
		}
	case "output_format":
		if value != "table" && value != "json" {
			return fmt.Errorf("invalid output format: %s (must be 'table' or 'json')", value)
		}
		viper.Set("output_format", value)
		if err := writeConfigFile(); err != nil {
			return err
		}
	case "client_id":
		viper.Set("client_id", value)
		if err := writeConfigFile(); err != nil {
			return err
		}
	case "client_secret":
		viper.Set("client_secret", value)
		if err := writeConfigFile(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown configuration key: %s", key)
	}

	output.Success("Set '%s' to '%s'", key, maskIfSecret(key, value))
	return nil
}

func runConfigPath(cmd *cobra.Command, args []string) error {
	dir, err := config.ConfigDir()
	if err != nil {
		return err
	}
	fmt.Printf("%s/config.yaml\n", dir)
	return nil
}

func getValueWithSource(key, configValue, envVar string) string {
	if envVar != "" {
		if envValue := os.Getenv(envVar); envValue != "" {
			return envValue
		}
	}
	if configValue != "" {
		return configValue
	}
	return ""
}

func getSource(key, envVar string) string {
	if envVar != "" {
		if os.Getenv(envVar) != "" {
			return "env"
		}
	}
	if viper.IsSet(key) {
		return "config"
	}
	return "default"
}

func maskIfNotEmpty(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 4 {
		return "****"
	}
	return value[:4] + "****"
}

func maskSecret(value string) string {
	if value == "" {
		return ""
	}
	return "********"
}

func maskIfSecret(key, value string) string {
	if key == "client_secret" {
		return "********"
	}
	return value
}

func writeConfigFile() error {
	dir, err := config.ConfigDir()
	if err != nil {
		return err
	}
	configPath := fmt.Sprintf("%s/config.yaml", dir)
	return viper.WriteConfigAs(configPath)
}
