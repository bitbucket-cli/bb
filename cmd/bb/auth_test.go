package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/tom/bb/internal/auth"
	"github.com/tom/bb/internal/config"
)

// resetAuthMocks restores default implementations
func resetAuthMocks() {
	authGetClientID = config.GetClientID
	authGetClientSecret = config.GetClientSecret
	authLogin = auth.Login
	authStoreTokens = auth.StoreTokens
	authLogout = auth.Logout
	authGetStatus = auth.GetStatus
	authSetClientCreds = config.SetClientCredentials
	authStdinReader = os.Stdin
}

func TestRunAuthStatus_NotLoggedIn(t *testing.T) {
	// Set up test environment
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	os.Unsetenv("BITBUCKET_API_TOKEN")
	auth.ForceFileStorage = true
	defer func() {
		os.Setenv("HOME", origHome)
		auth.ForceFileStorage = false
	}()

	// Initialize config
	config.Init()

	// Ensure no tokens
	auth.DeleteTokens()

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runAuthStatus(authStatusCmd, []string{})

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)

	if err != nil {
		t.Fatalf("runAuthStatus error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Not logged in") {
		t.Errorf("Output should contain 'Not logged in', got: %s", output)
	}
}

func TestRunAuthStatus_WithAPIToken(t *testing.T) {
	os.Setenv("BITBUCKET_API_TOKEN", "test-token")
	defer os.Unsetenv("BITBUCKET_API_TOKEN")

	// Initialize config
	config.Init()

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runAuthStatus(authStatusCmd, []string{})

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)

	if err != nil {
		t.Fatalf("runAuthStatus error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Logged in") {
		t.Errorf("Output should contain 'Logged in', got: %s", output)
	}
	if !strings.Contains(output, "BITBUCKET_API_TOKEN") {
		t.Errorf("Output should mention BITBUCKET_API_TOKEN, got: %s", output)
	}
}

func TestRunAuthStatus_WithOAuth(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	os.Unsetenv("BITBUCKET_API_TOKEN")
	auth.ForceFileStorage = true
	defer func() {
		os.Setenv("HOME", origHome)
		auth.ForceFileStorage = false
	}()

	config.Init()

	// Store OAuth tokens
	auth.StoreTokens(&auth.TokenData{
		AccessToken: "oauth-token",
		Scopes:      "repo,pr",
	})

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runAuthStatus(authStatusCmd, []string{})

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)

	if err != nil {
		t.Fatalf("runAuthStatus error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Logged in") {
		t.Errorf("Output should contain 'Logged in', got: %s", output)
	}
	if !strings.Contains(output, "OAuth") {
		t.Errorf("Output should mention OAuth, got: %s", output)
	}
}

func TestRunAuthLogout(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	auth.ForceFileStorage = true
	defer func() {
		os.Setenv("HOME", origHome)
		auth.ForceFileStorage = false
	}()

	config.Init()

	// Store tokens first
	auth.StoreTokens(&auth.TokenData{AccessToken: "test"})

	// Verify tokens exist
	if !auth.HasStoredTokens() {
		t.Fatal("Tokens should exist before logout")
	}

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runAuthLogout(authLogoutCmd, []string{})

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)

	if err != nil {
		t.Fatalf("runAuthLogout error: %v", err)
	}

	// Output will be either "Logged out" or "Already logged out" depending on keyring state
	output := buf.String()
	if !strings.Contains(output, "logged out") && !strings.Contains(output, "Logged out") {
		t.Errorf("Output should contain logged out message, got: %s", output)
	}
}

func TestRunAuthLogout_AlreadyLoggedOut(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	auth.ForceFileStorage = true
	defer func() {
		os.Setenv("HOME", origHome)
		auth.ForceFileStorage = false
	}()

	config.Init()

	// Ensure no tokens
	auth.DeleteTokens()

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runAuthLogout(authLogoutCmd, []string{})

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)

	if err != nil {
		t.Fatalf("runAuthLogout error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Already logged out") {
		t.Errorf("Output should contain 'Already logged out', got: %s", output)
	}
}

func TestRunAuthLogin_NoCredentials(t *testing.T) {
	// Override credential lookups so viper's global state doesn't leak in
	origGetClientID := authGetClientID
	origGetClientSecret := authGetClientSecret
	authGetClientID = func() string { return "" }
	authGetClientSecret = func() string { return "" }
	defer func() {
		authGetClientID = origGetClientID
		authGetClientSecret = origGetClientSecret
	}()

	// Simulate empty input so the prompts return empty strings
	r, w, _ := os.Pipe()
	w.Close()
	origStdinReader := authStdinReader
	authStdinReader = r
	defer func() { authStdinReader = origStdinReader }()

	err := runAuthLogin(authLoginCmd, []string{})

	if err == nil {
		t.Error("runAuthLogin should fail without credentials")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("Error should mention 'required', got: %v", err)
	}
}

func TestAuthCommandStructure(t *testing.T) {
	// Test login command
	if authLoginCmd.Short == "" {
		t.Error("login command should have short description")
	}
	if authLoginCmd.Long == "" {
		t.Error("login command should have long description")
	}
	if authLoginCmd.RunE == nil {
		t.Error("login command should have RunE function")
	}

	// Test logout command
	if authLogoutCmd.Short == "" {
		t.Error("logout command should have short description")
	}
	if authLogoutCmd.RunE == nil {
		t.Error("logout command should have RunE function")
	}

	// Test status command
	if authStatusCmd.Short == "" {
		t.Error("status command should have short description")
	}
	if authStatusCmd.RunE == nil {
		t.Error("status command should have RunE function")
	}
}

func TestRunAuthLogin_WithCredentials_LoginFails(t *testing.T) {
	defer resetAuthMocks()

	authGetClientID = func() string { return "test-id" }
	authGetClientSecret = func() string { return "test-secret" }
	authLogin = func(cfg auth.OAuthConfig) (*auth.TokenData, error) {
		return nil, errors.New("login failed")
	}

	err := runAuthLogin(authLoginCmd, []string{})
	if err == nil {
		t.Error("Expected error when login fails")
	}
	if !strings.Contains(err.Error(), "login failed") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunAuthLogin_WithCredentials_StoreTokensFails(t *testing.T) {
	defer resetAuthMocks()

	authGetClientID = func() string { return "test-id" }
	authGetClientSecret = func() string { return "test-secret" }
	authLogin = func(cfg auth.OAuthConfig) (*auth.TokenData, error) {
		return &auth.TokenData{AccessToken: "token"}, nil
	}
	authStoreTokens = func(tokens *auth.TokenData) error {
		return errors.New("store failed")
	}

	err := runAuthLogin(authLoginCmd, []string{})
	if err == nil {
		t.Error("Expected error when storing tokens fails")
	}
	if !strings.Contains(err.Error(), "failed to store tokens") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunAuthLogin_WithCredentials_Success(t *testing.T) {
	defer resetAuthMocks()

	authGetClientID = func() string { return "test-id" }
	authGetClientSecret = func() string { return "test-secret" }
	authLogin = func(cfg auth.OAuthConfig) (*auth.TokenData, error) {
		return &auth.TokenData{AccessToken: "token"}, nil
	}
	authStoreTokens = func(tokens *auth.TokenData) error {
		return nil
	}

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runAuthLogin(authLoginCmd, []string{})

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Logged in") {
		t.Errorf("Expected success message, got: %s", output)
	}
}

func TestRunAuthLogin_PromptForCredentials_SaveSuccess(t *testing.T) {
	defer resetAuthMocks()

	authGetClientID = func() string { return "" }
	authGetClientSecret = func() string { return "" }
	authStdinReader = strings.NewReader("my-client-id\nmy-secret\n")
	authSetClientCreds = func(id, secret string) error {
		return nil
	}
	authLogin = func(cfg auth.OAuthConfig) (*auth.TokenData, error) {
		return &auth.TokenData{AccessToken: "token"}, nil
	}
	authStoreTokens = func(tokens *auth.TokenData) error {
		return nil
	}

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runAuthLogin(authLoginCmd, []string{})

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Credentials saved") {
		t.Errorf("Expected save success message, got: %s", output)
	}
}

func TestRunAuthLogin_PromptForCredentials_SaveFails(t *testing.T) {
	defer resetAuthMocks()

	authGetClientID = func() string { return "" }
	authGetClientSecret = func() string { return "" }
	authStdinReader = strings.NewReader("my-client-id\nmy-secret\n")
	authSetClientCreds = func(id, secret string) error {
		return errors.New("save failed")
	}
	authLogin = func(cfg auth.OAuthConfig) (*auth.TokenData, error) {
		return &auth.TokenData{AccessToken: "token"}, nil
	}
	authStoreTokens = func(tokens *auth.TokenData) error {
		return nil
	}

	// Capture both stdout and stderr
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	err := runAuthLogin(authLoginCmd, []string{})

	wOut.Close()
	wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var bufOut, bufErr bytes.Buffer
	io.Copy(&bufOut, rOut)
	io.Copy(&bufErr, rErr)

	// Should still succeed even if save fails
	if err != nil {
		t.Errorf("Should succeed even if save fails: %v", err)
	}

	// Check stderr for error message
	errOutput := bufErr.String()
	if !strings.Contains(errOutput, "Failed to save credentials") {
		t.Errorf("Expected warning about save failure in stderr, got: %s", errOutput)
	}
}

func TestRunAuthLogout_Success(t *testing.T) {
	defer resetAuthMocks()

	authLogout = func() error {
		return nil
	}

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runAuthLogout(authLogoutCmd, []string{})

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Logged out") {
		t.Errorf("Expected logout success message, got: %s", output)
	}
}

func TestRunAuthLogout_NotLoggedIn(t *testing.T) {
	defer resetAuthMocks()

	authLogout = func() error {
		return errors.New("not logged in")
	}

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runAuthLogout(authLogoutCmd, []string{})

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)

	if err != nil {
		t.Errorf("Should not return error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Already logged out") {
		t.Errorf("Expected already logged out message, got: %s", output)
	}
}

func TestRunAuthStatus_Error(t *testing.T) {
	defer resetAuthMocks()

	authGetStatus = func() (*auth.AuthStatus, error) {
		return nil, errors.New("status error")
	}

	err := runAuthStatus(authStatusCmd, []string{})
	if err == nil {
		t.Error("Expected error from status")
	}
}

func TestRunAuthStatus_NotLoggedIn_Mocked(t *testing.T) {
	defer resetAuthMocks()

	authGetStatus = func() (*auth.AuthStatus, error) {
		return &auth.AuthStatus{LoggedIn: false}, nil
	}

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runAuthStatus(authStatusCmd, []string{})

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Not logged in") {
		t.Errorf("Expected not logged in message, got: %s", output)
	}
}

func TestRunAuthStatus_Token_Mocked(t *testing.T) {
	defer resetAuthMocks()

	authGetStatus = func() (*auth.AuthStatus, error) {
		return &auth.AuthStatus{
			LoggedIn: true,
			Method:   auth.AuthMethodToken,
		}, nil
	}

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runAuthStatus(authStatusCmd, []string{})

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "BITBUCKET_API_TOKEN") {
		t.Errorf("Expected token method message, got: %s", output)
	}
}

func TestRunAuthStatus_OAuth_Mocked(t *testing.T) {
	defer resetAuthMocks()

	authGetStatus = func() (*auth.AuthStatus, error) {
		return &auth.AuthStatus{
			LoggedIn: true,
			Method:   auth.AuthMethodOAuth,
			Scopes:   "repo,pr",
		}, nil
	}

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runAuthStatus(authStatusCmd, []string{})

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "OAuth") {
		t.Errorf("Expected OAuth method message, got: %s", output)
	}
	if !strings.Contains(output, "repo,pr") {
		t.Errorf("Expected scopes in output, got: %s", output)
	}
}

func TestRunAuthStatus_OAuth_NoScopes(t *testing.T) {
	defer resetAuthMocks()

	authGetStatus = func() (*auth.AuthStatus, error) {
		return &auth.AuthStatus{
			LoggedIn: true,
			Method:   auth.AuthMethodOAuth,
			Scopes:   "",
		}, nil
	}

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runAuthStatus(authStatusCmd, []string{})

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "OAuth") {
		t.Errorf("Expected OAuth method message, got: %s", output)
	}
}

func TestAuthSubcommands(t *testing.T) {
	subCmds := authCmd.Commands()
	expectedCmds := []string{"login", "logout", "status"}

	for _, expected := range expectedCmds {
		found := false
		for _, cmd := range subCmds {
			if cmd.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected auth subcommand %q not found", expected)
		}
	}
}
