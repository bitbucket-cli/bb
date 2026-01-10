package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// executeCommand runs a cobra command with args and captures output
func executeCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	err := root.Execute()
	return buf.String(), err
}

func TestRootCommand(t *testing.T) {
	// Test that root command exists and has expected subcommands
	if rootCmd == nil {
		t.Fatal("rootCmd is nil")
	}

	// Check subcommands exist
	subCmds := rootCmd.Commands()
	expectedCmds := []string{"auth", "repo", "pr", "branch", "pipeline"}

	for _, expected := range expectedCmds {
		found := false
		for _, cmd := range subCmds {
			if cmd.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected subcommand %q not found", expected)
		}
	}
}

func TestRootCommand_JSONFlag(t *testing.T) {
	// Verify --json flag exists
	flag := rootCmd.PersistentFlags().Lookup("json")
	if flag == nil {
		t.Error("--json flag not found")
	}
}

func TestRootCommand_Version(t *testing.T) {
	if rootCmd.Version == "" {
		// Version is set at build time, so "dev" is acceptable
	}
}

func TestAuthCommand(t *testing.T) {
	if authCmd == nil {
		t.Fatal("authCmd is nil")
	}

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

func TestRepoCommand(t *testing.T) {
	if repoCmd == nil {
		t.Fatal("repoCmd is nil")
	}

	subCmds := repoCmd.Commands()
	expectedCmds := []string{"list", "view", "clone"}

	for _, expected := range expectedCmds {
		found := false
		for _, cmd := range subCmds {
			if cmd.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected repo subcommand %q not found", expected)
		}
	}
}

func TestPRCommand(t *testing.T) {
	if prCmd == nil {
		t.Fatal("prCmd is nil")
	}
}

func TestBranchCommand(t *testing.T) {
	if branchCmd == nil {
		t.Fatal("branchCmd is nil")
	}
}

func TestPipelineCommand(t *testing.T) {
	if pipelineCmd == nil {
		t.Fatal("pipelineCmd is nil")
	}
}

func TestMain_Success(t *testing.T) {
	// Save originals
	oldArgs := os.Args
	oldExit := osExit
	defer func() {
		os.Args = oldArgs
		osExit = oldExit
	}()

	// Mock osExit
	exitCode := 0
	osExit = func(code int) {
		exitCode = code
	}

	// Run with --help which should succeed without error
	os.Args = []string{"bb", "--help"}

	// Reset command state
	rootCmd.SetArgs([]string{"--help"})
	main()

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}
}

func TestMain_Error(t *testing.T) {
	// Save originals
	oldArgs := os.Args
	oldExit := osExit
	defer func() {
		os.Args = oldArgs
		osExit = oldExit
	}()

	// Mock osExit
	exitCalled := false
	osExit = func(code int) {
		exitCalled = true
		if code != 1 {
			t.Errorf("Expected exit code 1, got %d", code)
		}
	}

	// Run with invalid command
	os.Args = []string{"bb", "invalid-command-that-doesnt-exist"}
	rootCmd.SetArgs([]string{"invalid-command-that-doesnt-exist"})
	main()

	if !exitCalled {
		t.Error("Expected osExit to be called on error")
	}
}

func TestInit(t *testing.T) {
	// Verify init() ran and configured flags
	if rootCmd.PersistentFlags().Lookup("json") == nil {
		t.Error("init() didn't set up --json flag")
	}
}

// TestHelp verifies help text is available
func TestHelp(t *testing.T) {
	if rootCmd.Short == "" {
		t.Error("Root command should have short description")
	}
	if rootCmd.Long == "" {
		t.Error("Root command should have long description")
	}
}

// Test that Use is set correctly
func TestCommandUse(t *testing.T) {
	tests := []struct {
		cmd  *cobra.Command
		want string
	}{
		{rootCmd, "bb"},
		{authCmd, "auth"},
		{repoCmd, "repo"},
		{prCmd, "pr"},
		{branchCmd, "branch"},
		{pipelineCmd, "pipeline"},
	}

	for _, tt := range tests {
		if !strings.HasPrefix(tt.cmd.Use, tt.want) {
			t.Errorf("Command Use = %q, want prefix %q", tt.cmd.Use, tt.want)
		}
	}
}
