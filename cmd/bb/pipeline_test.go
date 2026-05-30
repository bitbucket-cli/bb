package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ThomasRedstone/bb/generated/bitbucket"
	"github.com/ThomasRedstone/bb/internal/api"
)

// resetPipelineMocks restores default implementations
func resetPipelineMocks() {
	pipelineNewAPIClient = api.NewClient
}

func TestRunPipelineList_NoClient(t *testing.T) {
	defer resetPipelineMocks()

	pipelineNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runPipelineList(pipelineListCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunPipelineList_InvalidRepo(t *testing.T) {
	defer resetPipelineMocks()

	pipelineNewAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runPipelineList(pipelineListCmd, []string{"invalid"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestRunPipelineView_NoClient(t *testing.T) {
	defer resetPipelineMocks()

	pipelineNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runPipelineView(pipelineViewCmd, []string{"workspace/repo", "uuid"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunPipelineView_InvalidRepo(t *testing.T) {
	defer resetPipelineMocks()

	pipelineNewAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runPipelineView(pipelineViewCmd, []string{"invalid", "uuid"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestRunPipelineTrigger_NoClient(t *testing.T) {
	defer resetPipelineMocks()

	pipelineNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runPipelineTrigger(pipelineTriggerCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunPipelineTrigger_InvalidRepo(t *testing.T) {
	defer resetPipelineMocks()

	pipelineNewAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runPipelineTrigger(pipelineTriggerCmd, []string{"invalid"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestRunPipelineStop_NoClient(t *testing.T) {
	defer resetPipelineMocks()

	pipelineNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runPipelineStop(pipelineStopCmd, []string{"workspace/repo", "uuid"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunPipelineStop_InvalidRepo(t *testing.T) {
	defer resetPipelineMocks()

	pipelineNewAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runPipelineStop(pipelineStopCmd, []string{"invalid", "uuid"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestPipelineCommandStructure(t *testing.T) {
	if pipelineCmd.Short == "" {
		t.Error("pipeline command should have short description")
	}

	if pipelineListCmd.RunE == nil {
		t.Error("pipeline list should have RunE")
	}

	if pipelineViewCmd.RunE == nil {
		t.Error("pipeline view should have RunE")
	}

	if pipelineTriggerCmd.RunE == nil {
		t.Error("pipeline trigger should have RunE")
	}

	if pipelineStopCmd.RunE == nil {
		t.Error("pipeline stop should have RunE")
	}
}

func TestPipelineFlags(t *testing.T) {
	// Test --branch flag exists on trigger command
	branchFlag := pipelineTriggerCmd.Flags().Lookup("branch")
	if branchFlag == nil {
		t.Error("--branch flag not found on pipeline trigger command")
	}
}

func TestPipelineSubcommands(t *testing.T) {
	subCmds := pipelineCmd.Commands()
	expectedCmds := []string{"list", "view", "trigger", "stop"}

	for _, expected := range expectedCmds {
		found := false
		for _, cmd := range subCmds {
			if cmd.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected pipeline subcommand %q not found", expected)
		}
	}
}

func TestPipelineAliases(t *testing.T) {
	if len(pipelineCmd.Aliases) == 0 {
		t.Error("pipeline command should have aliases")
	}

	found := false
	for _, alias := range pipelineCmd.Aliases {
		if alias == "pipelines" {
			found = true
			break
		}
	}
	if !found {
		t.Error("pipeline command should have 'pipelines' alias")
	}
}

func TestGetPipelineStatus(t *testing.T) {
	tests := []struct {
		name     string
		pipeline *bitbucket.Pipeline
		want     string
	}{
		{
			name:     "nil state",
			pipeline: &bitbucket.Pipeline{State: nil},
			want:     "unknown",
		},
		{
			name: "empty state type",
			pipeline: &bitbucket.Pipeline{
				State: &bitbucket.PipelineState{Type: ""},
			},
			want: "unknown",
		},
		{
			name: "state with type only",
			pipeline: &bitbucket.Pipeline{
				State: &bitbucket.PipelineState{Type: "pending"},
			},
			want: "pending",
		},
		{
			name: "state with name in additional properties",
			pipeline: &bitbucket.Pipeline{
				State: &bitbucket.PipelineState{
					Type:                 "some_type",
					AdditionalProperties: map[string]interface{}{"name": "IN_PROGRESS"},
				},
			},
			want: "IN_PROGRESS",
		},
		{
			name: "state with result",
			pipeline: &bitbucket.Pipeline{
				State: &bitbucket.PipelineState{
					Type: "completed",
					AdditionalProperties: map[string]interface{}{
						"result": map[string]interface{}{"name": "SUCCESSFUL"},
					},
				},
			},
			want: "SUCCESSFUL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPipelineStatus(tt.pipeline)
			if got != tt.want {
				t.Errorf("getPipelineStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetPipelineTarget(t *testing.T) {
	tests := []struct {
		name     string
		pipeline *bitbucket.Pipeline
		want     string
	}{
		{
			name:     "nil target",
			pipeline: &bitbucket.Pipeline{Target: nil},
			want:     "",
		},
		{
			name: "target with ref_name",
			pipeline: &bitbucket.Pipeline{
				Target: &bitbucket.PipelineTarget{
					AdditionalProperties: map[string]interface{}{"ref_name": "main"},
				},
			},
			want: "main",
		},
		{
			name: "target without ref_name",
			pipeline: &bitbucket.Pipeline{
				Target: &bitbucket.PipelineTarget{
					AdditionalProperties: map[string]interface{}{},
				},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPipelineTarget(tt.pipeline)
			if got != tt.want {
				t.Errorf("getPipelineTarget() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		seconds int
		want    string
	}{
		{0, "0s"},
		{30, "30s"},
		{59, "59s"},
		{60, "1m 0s"},
		{90, "1m 30s"},
		{3599, "59m 59s"},
		{3600, "1h 0m"},
		{3660, "1h 1m"},
		{7200, "2h 0m"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatDuration(tt.seconds)
			if got != tt.want {
				t.Errorf("formatDuration(%d) = %v, want %v", tt.seconds, got, tt.want)
			}
		})
	}
}

func TestRunPipelineList_APIError(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineList(pipelineListCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunPipelineList_NoPipelines(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := bitbucket.PaginatedPipelines{}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineList(pipelineListCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunPipelineList_WithPipelines(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buildNum := 42
		uuid := "{uuid}"
		createdOn := time.Now()
		duration := 120
		pipelines := []bitbucket.Pipeline{
			{
				BuildNumber:      &buildNum,
				Uuid:             &uuid,
				CreatedOn:        &createdOn,
				BuildSecondsUsed: &duration,
				State: &bitbucket.PipelineState{
					Type:                 "completed",
					AdditionalProperties: map[string]interface{}{"name": "SUCCESSFUL"},
				},
				Target: &bitbucket.PipelineTarget{
					AdditionalProperties: map[string]interface{}{"ref_name": "main"},
				},
			},
		}
		result := bitbucket.PaginatedPipelines{Values: &pipelines}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineList(pipelineListCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunPipelineList_InvalidJSON(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineList(pipelineListCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error on invalid JSON")
	}
}

func TestRunPipelineView_APIError(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineView(pipelineViewCmd, []string{"workspace/repo", "uuid"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunPipelineView_Success(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buildNum := 42
		uuid := "{uuid}"
		createdOn := time.Now()
		completedOn := time.Now()
		duration := 120
		displayName := "Test User"
		pipeline := bitbucket.Pipeline{
			BuildNumber:      &buildNum,
			Uuid:             &uuid,
			CreatedOn:        &createdOn,
			CompletedOn:      &completedOn,
			BuildSecondsUsed: &duration,
			State: &bitbucket.PipelineState{
				Type:                 "completed",
				AdditionalProperties: map[string]interface{}{"name": "SUCCESSFUL"},
			},
			Target: &bitbucket.PipelineTarget{
				AdditionalProperties: map[string]interface{}{"ref_name": "main"},
			},
			Creator: &bitbucket.Account{
				DisplayName: &displayName,
			},
		}
		json.NewEncoder(w).Encode(pipeline)
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineView(pipelineViewCmd, []string{"workspace/repo", "uuid"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunPipelineView_WithBraces(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buildNum := 42
		pipeline := bitbucket.Pipeline{BuildNumber: &buildNum}
		json.NewEncoder(w).Encode(pipeline)
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	// UUID already has braces
	err := runPipelineView(pipelineViewCmd, []string{"workspace/repo", "{uuid}"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunPipelineView_InvalidJSON(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineView(pipelineViewCmd, []string{"workspace/repo", "uuid"})
	if err == nil {
		t.Error("Expected error on invalid JSON")
	}
}

func TestRunPipelineView_JSONOutput(t *testing.T) {
	defer resetPipelineMocks()
	origJSONOutput := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = origJSONOutput }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buildNum := 42
		pipeline := bitbucket.Pipeline{BuildNumber: &buildNum}
		json.NewEncoder(w).Encode(pipeline)
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineView(pipelineViewCmd, []string{"workspace/repo", "uuid"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunPipelineTrigger_APIError(t *testing.T) {
	defer resetPipelineMocks()
	origBranch := pipelineBranch
	pipelineBranch = "main"
	defer func() { pipelineBranch = origBranch }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "bad request"})
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineTrigger(pipelineTriggerCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunPipelineTrigger_Success(t *testing.T) {
	defer resetPipelineMocks()
	origBranch := pipelineBranch
	pipelineBranch = "main"
	defer func() { pipelineBranch = origBranch }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		buildNum := 42
		uuid := "{uuid}"
		pipeline := bitbucket.Pipeline{
			BuildNumber: &buildNum,
			Uuid:        &uuid,
		}
		json.NewEncoder(w).Encode(pipeline)
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineTrigger(pipelineTriggerCmd, []string{"workspace/repo"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunPipelineTrigger_InvalidResponseJSON(t *testing.T) {
	defer resetPipelineMocks()
	origBranch := pipelineBranch
	pipelineBranch = "main"
	defer func() { pipelineBranch = origBranch }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineTrigger(pipelineTriggerCmd, []string{"workspace/repo"})
	if err == nil {
		t.Error("Expected error on invalid JSON")
	}
}

func TestRunPipelineStop_APIError(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "not found"})
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineStop(pipelineStopCmd, []string{"workspace/repo", "uuid"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunPipelineStop_Success(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineStop(pipelineStopCmd, []string{"workspace/repo", "uuid"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunPipelineStop_WithBraces(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineStop(pipelineStopCmd, []string{"workspace/repo", "{uuid}"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

// --- Pipeline Steps Tests ---

func TestPipelineStepsSubcommands(t *testing.T) {
	subCmds := pipelineCmd.Commands()
	expectedCmds := []string{"list", "view", "trigger", "stop", "steps", "logs"}

	for _, expected := range expectedCmds {
		found := false
		for _, cmd := range subCmds {
			if cmd.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected pipeline subcommand %q not found", expected)
		}
	}
}

func TestRunPipelineSteps_NoClient(t *testing.T) {
	defer resetPipelineMocks()

	pipelineNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runPipelineSteps(pipelineStepsCmd, []string{"workspace/repo", "uuid"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunPipelineSteps_InvalidRepo(t *testing.T) {
	defer resetPipelineMocks()

	pipelineNewAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runPipelineSteps(pipelineStepsCmd, []string{"invalid", "uuid"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestRunPipelineSteps_APIError(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineSteps(pipelineStepsCmd, []string{"workspace/repo", "uuid"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunPipelineSteps_NoSteps(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"values": []interface{}{}})
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineSteps(pipelineStepsCmd, []string{"workspace/repo", "uuid"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunPipelineSteps_WithSteps(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		duration := 60
		result := map[string]interface{}{
			"values": []interface{}{
				map[string]interface{}{
					"uuid": "{step-uuid}",
					"name": "Build",
					"state": map[string]interface{}{
						"name": "COMPLETED",
						"result": map[string]interface{}{
							"name": "SUCCESSFUL",
						},
					},
					"duration_in_seconds": duration,
				},
			},
		}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineSteps(pipelineStepsCmd, []string{"workspace/repo", "uuid"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunPipelineSteps_WithBraces(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"values": []interface{}{}})
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineSteps(pipelineStepsCmd, []string{"workspace/repo", "{uuid}"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

// --- Pipeline Logs Tests ---

func TestRunPipelineLogs_NoClient(t *testing.T) {
	defer resetPipelineMocks()

	pipelineNewAPIClient = func() (*api.Client, error) {
		return nil, errors.New("auth required")
	}

	err := runPipelineLogs(pipelineLogsCmd, []string{"workspace/repo", "pipeline-uuid", "step-uuid"})
	if err == nil {
		t.Error("Expected error when client creation fails")
	}
}

func TestRunPipelineLogs_InvalidRepo(t *testing.T) {
	defer resetPipelineMocks()

	pipelineNewAPIClient = func() (*api.Client, error) {
		return &api.Client{}, nil
	}

	err := runPipelineLogs(pipelineLogsCmd, []string{"invalid", "pipeline-uuid", "step-uuid"})
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestRunPipelineLogs_APIError(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineLogs(pipelineLogsCmd, []string{"workspace/repo", "pipeline-uuid", "step-uuid"})
	if err == nil {
		t.Error("Expected error on API error")
	}
}

func TestRunPipelineLogs_Success(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Build started...\nCompiling...\nBuild successful!"))
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineLogs(pipelineLogsCmd, []string{"workspace/repo", "pipeline-uuid", "step-uuid"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRunPipelineLogs_WithBraces(t *testing.T) {
	defer resetPipelineMocks()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Log content"))
	}))
	defer server.Close()

	pipelineNewAPIClient = mockAPIClientFactory(server)

	err := runPipelineLogs(pipelineLogsCmd, []string{"workspace/repo", "{pipeline-uuid}", "{step-uuid}"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}
