package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/spf13/viper"
)

// resetState resets the global config state for testing
func resetState() {
	cfg = nil
	viper.Reset()
	homeDirFunc = os.UserHomeDir // Reset to default
}

func TestConfigDir(t *testing.T) {
	dir, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir() error: %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config", "bb")
	if dir != expected {
		t.Errorf("ConfigDir() = %q, want %q", dir, expected)
	}
}

func TestInit(t *testing.T) {
	resetState()

	// Use a temp directory for config
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", tmpDir)
	}
	defer os.Setenv("HOME", origHome)

	err := Init()
	if err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	// Config directory should be created
	configPath := filepath.Join(tmpDir, ".config", "bb")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Init() did not create config directory")
	}
}

func TestWriteConfig_SecretFilePermissions(t *testing.T) {
	resetState()

	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", tmpDir)
	}
	defer os.Setenv("HOME", origHome)

	if err := Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	// Writing client credentials must leave the config file owner-only (0600),
	// since it holds the OAuth client secret.
	if err := SetClientCredentials("id", "secret"); err != nil {
		t.Fatalf("SetClientCredentials error: %v", err)
	}

	configPath := filepath.Join(tmpDir, ".config", "bb", "config.yaml")
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if perm := info.Mode().Perm(); runtime.GOOS != "windows" && perm != 0600 {
		t.Errorf("config.yaml perms = %o, want 0600", perm)
	}
}

func TestSetGetScopes(t *testing.T) {
	resetState()

	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", tmpDir)
	}
	defer os.Setenv("HOME", origHome)
	os.Unsetenv("BB_SCOPES")

	if err := Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	if err := SetScopes("admin"); err != nil {
		t.Fatalf("SetScopes error: %v", err)
	}

	// Persisted to disk: a fresh load (next process) picks it up.
	if err := Init(); err != nil {
		t.Fatalf("reload Init() error: %v", err)
	}
	if got := Get().Scopes; got != "admin" {
		t.Errorf("after reload Get().Scopes = %q, want %q", got, "admin")
	}
	if got := GetScopes(); got != "admin" {
		t.Errorf("GetScopes() = %q, want %q", got, "admin")
	}

	// Env var takes precedence over the persisted value.
	os.Setenv("BB_SCOPES", "read")
	defer os.Unsetenv("BB_SCOPES")
	if got := GetScopes(); got != "read" {
		t.Errorf("GetScopes() = %q, want %q (env override)", got, "read")
	}
}

func TestGet_NilConfig(t *testing.T) {
	resetState()

	// When cfg is nil, Get() should return empty Config
	c := Get()
	if c == nil {
		t.Fatal("Get() returned nil")
	}
	if c.ClientID != "" || c.Workspace != "" {
		t.Error("Get() on nil config should return empty struct")
	}
}

func TestGetClientID_EnvOverride(t *testing.T) {
	resetState()

	// Set up config with a value
	cfg = &Config{ClientID: "config-id"}

	// Env var should take precedence
	os.Setenv("BB_CLIENT_ID", "env-id")
	defer os.Unsetenv("BB_CLIENT_ID")

	got := GetClientID()
	if got != "env-id" {
		t.Errorf("GetClientID() = %q, want %q", got, "env-id")
	}
}

func TestGetClientID_FromConfig(t *testing.T) {
	resetState()

	// Ensure env var is not set
	os.Unsetenv("BB_CLIENT_ID")

	cfg = &Config{ClientID: "config-id"}

	got := GetClientID()
	if got != "config-id" {
		t.Errorf("GetClientID() = %q, want %q", got, "config-id")
	}
}

func TestGetClientSecret_EnvOverride(t *testing.T) {
	resetState()

	cfg = &Config{ClientSecret: "config-secret"}

	os.Setenv("BB_CLIENT_SECRET", "env-secret")
	defer os.Unsetenv("BB_CLIENT_SECRET")

	got := GetClientSecret()
	if got != "env-secret" {
		t.Errorf("GetClientSecret() = %q, want %q", got, "env-secret")
	}
}

func TestGetClientSecret_FromConfig(t *testing.T) {
	resetState()

	os.Unsetenv("BB_CLIENT_SECRET")
	cfg = &Config{ClientSecret: "config-secret"}

	got := GetClientSecret()
	if got != "config-secret" {
		t.Errorf("GetClientSecret() = %q, want %q", got, "config-secret")
	}
}

func TestGetWorkspace_EnvOverride(t *testing.T) {
	resetState()

	cfg = &Config{Workspace: "config-ws"}

	os.Setenv("BB_WORKSPACE", "env-ws")
	defer os.Unsetenv("BB_WORKSPACE")

	got := GetWorkspace()
	if got != "env-ws" {
		t.Errorf("GetWorkspace() = %q, want %q", got, "env-ws")
	}
}

func TestGetWorkspace_FromConfig(t *testing.T) {
	resetState()

	os.Unsetenv("BB_WORKSPACE")
	cfg = &Config{Workspace: "config-ws"}

	got := GetWorkspace()
	if got != "config-ws" {
		t.Errorf("GetWorkspace() = %q, want %q", got, "config-ws")
	}
}

func TestHasClientCredentials(t *testing.T) {
	tests := []struct {
		name     string
		clientID string
		secret   string
		envID    string
		envSec   string
		want     bool
	}{
		{"both set in config", "id", "secret", "", "", true},
		{"both set in env", "", "", "id", "secret", true},
		{"mixed config and env", "id", "", "", "secret", true},
		{"only id", "id", "", "", "", false},
		{"only secret", "", "secret", "", "", false},
		{"neither", "", "", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetState()
			cfg = &Config{ClientID: tt.clientID, ClientSecret: tt.secret}

			os.Unsetenv("BB_CLIENT_ID")
			os.Unsetenv("BB_CLIENT_SECRET")
			if tt.envID != "" {
				os.Setenv("BB_CLIENT_ID", tt.envID)
				defer os.Unsetenv("BB_CLIENT_ID")
			}
			if tt.envSec != "" {
				os.Setenv("BB_CLIENT_SECRET", tt.envSec)
				defer os.Unsetenv("BB_CLIENT_SECRET")
			}

			got := HasClientCredentials()
			if got != tt.want {
				t.Errorf("HasClientCredentials() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetClientCredentials(t *testing.T) {
	resetState()

	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", tmpDir)
	}
	defer os.Setenv("HOME", origHome)

	// Initialize config first
	if err := Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	err := SetClientCredentials("new-id", "new-secret")
	if err != nil {
		t.Fatalf("SetClientCredentials() error: %v", err)
	}

	// Verify config file was written
	configPath := filepath.Join(tmpDir, ".config", "bb", "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("SetClientCredentials() did not write config file")
	}
}

func TestSetWorkspace(t *testing.T) {
	resetState()

	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", tmpDir)
	}
	defer os.Setenv("HOME", origHome)

	if err := Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	err := SetWorkspace("my-workspace")
	if err != nil {
		t.Fatalf("SetWorkspace() error: %v", err)
	}

	// Verify config file was written
	configPath := filepath.Join(tmpDir, ".config", "bb", "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("SetWorkspace() did not write config file")
	}
}

func TestInit_WithExistingConfig(t *testing.T) {
	resetState()

	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", tmpDir)
	}
	defer os.Setenv("HOME", origHome)

	// Create config directory and file
	configDir := filepath.Join(tmpDir, ".config", "bb")
	os.MkdirAll(configDir, 0700)

	configContent := `client_id: file-id
client_secret: file-secret
workspace: file-ws
`
	os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configContent), 0600)

	// Clear env vars to test file loading
	os.Unsetenv("BB_CLIENT_ID")
	os.Unsetenv("BB_CLIENT_SECRET")
	os.Unsetenv("BB_WORKSPACE")

	err := Init()
	if err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	// Verify values were loaded from file
	if GetClientID() != "file-id" {
		t.Errorf("GetClientID() = %q, want %q", GetClientID(), "file-id")
	}
	if GetClientSecret() != "file-secret" {
		t.Errorf("GetClientSecret() = %q, want %q", GetClientSecret(), "file-secret")
	}
	if GetWorkspace() != "file-ws" {
		t.Errorf("GetWorkspace() = %q, want %q", GetWorkspace(), "file-ws")
	}
}

func TestInit_InvalidConfigFile(t *testing.T) {
	resetState()

	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", tmpDir)
	}
	defer os.Setenv("HOME", origHome)

	// Create config directory with invalid YAML
	configDir := filepath.Join(tmpDir, ".config", "bb")
	os.MkdirAll(configDir, 0700)
	os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("invalid: yaml: content: ["), 0600)

	err := Init()
	if err == nil {
		t.Error("Init() with invalid config should return error")
	}
}

func TestInit_CannotCreateConfigDir(t *testing.T) {
	resetState()

	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", tmpDir)
	}
	defer os.Setenv("HOME", origHome)

	// Create a file where the config directory should be (blocks mkdir)
	configPath := filepath.Join(tmpDir, ".config", "bb")
	os.MkdirAll(filepath.Join(tmpDir, ".config"), 0700)
	os.WriteFile(configPath, []byte("blocker"), 0600)

	err := Init()
	if err == nil {
		t.Error("Init() should fail when cannot create config directory")
	}
}

func TestWriteConfig_Error(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows chmod does not make directories read-only")
	}
	resetState()

	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", tmpDir)
	}
	defer os.Setenv("HOME", origHome)

	if err := Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	// Make config directory read-only to cause write error
	configDir := filepath.Join(tmpDir, ".config", "bb")
	os.Chmod(configDir, 0500)
	defer os.Chmod(configDir, 0700) // Restore for cleanup

	err := SetWorkspace("test")
	if err == nil {
		t.Error("SetWorkspace() should fail when config dir is read-only")
	}
}

func TestConfigDir_HomeDirError(t *testing.T) {
	resetState()

	// Mock homeDirFunc to return an error
	homeDirFunc = func() (string, error) {
		return "", errors.New("home dir not found")
	}
	defer func() { homeDirFunc = os.UserHomeDir }()

	_, err := ConfigDir()
	if err == nil {
		t.Error("ConfigDir() should fail when home directory lookup fails")
	}
}

func TestInit_HomeDirError(t *testing.T) {
	resetState()

	homeDirFunc = func() (string, error) {
		return "", errors.New("home dir not found")
	}
	defer func() { homeDirFunc = os.UserHomeDir }()

	err := Init()
	if err == nil {
		t.Error("Init() should fail when home directory lookup fails")
	}
}

func TestWriteConfig_HomeDirError(t *testing.T) {
	resetState()

	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", tmpDir)
	}
	defer os.Setenv("HOME", origHome)

	// Initialize first
	if err := Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	// Then break homeDirFunc
	homeDirFunc = func() (string, error) {
		return "", errors.New("home dir not found")
	}
	defer func() { homeDirFunc = os.UserHomeDir }()

	err := SetWorkspace("test")
	if err == nil {
		t.Error("SetWorkspace() should fail when home directory lookup fails")
	}
}

func TestGetClientSecret_Command(t *testing.T) {
	os.Unsetenv("BB_CLIENT_SECRET")
	os.Setenv("BB_CLIENT_SECRET_COMMAND", "echo from-cmd")
	defer os.Unsetenv("BB_CLIENT_SECRET_COMMAND")
	if got := GetClientSecret(); got != "from-cmd" {
		t.Errorf("GetClientSecret via command = %q, want from-cmd", got)
	}
	// env value still wins over command
	os.Setenv("BB_CLIENT_SECRET", "from-env")
	defer os.Unsetenv("BB_CLIENT_SECRET")
	if got := GetClientSecret(); got != "from-env" {
		t.Errorf("env should win: got %q, want from-env", got)
	}
}

func TestCredentialShell(t *testing.T) {
	for _, tc := range []struct {
		goos, name string
		args       []string
	}{
		{"windows", "cmd.exe", []string{"/D", "/S", "/C", "echo credential"}},
		{"linux", "sh", []string{"-c", "echo credential"}},
		{"darwin", "sh", []string{"-c", "echo credential"}},
	} {
		name, args := credentialShell(tc.goos, "echo credential")
		if name != tc.name || !reflect.DeepEqual(args, tc.args) {
			t.Errorf("%s shell = %q %v", tc.goos, name, args)
		}
	}
}
func TestCredentialCommandFailure(t *testing.T) {
	if got := runCredentialCommand("exit 17"); got != "" {
		t.Errorf("failed command returned %q", got)
	}
}

func TestTokenStoreDefaultAndExplicitChoices(t *testing.T) {
	for _, tc := range []struct{ name, stored, env, want string }{
		{"default", "", "", "keyring"},
		{"existing file", "file", "", "file"},
		{"existing mirror", "auto", "", "auto"},
		{"environment", "file", "keyring", "keyring"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetState()
			t.Cleanup(resetState)
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			t.Setenv("BB_TOKEN_STORE", tc.env)
			if tc.stored != "" {
				dir := filepath.Join(home, ".config", "bb")
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("token_store: "+tc.stored+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := Init(); err != nil {
				t.Fatal(err)
			}
			if got := GetTokenStore(); got != tc.want {
				t.Fatalf("mode %q, want %q", got, tc.want)
			}
		})
	}
}
